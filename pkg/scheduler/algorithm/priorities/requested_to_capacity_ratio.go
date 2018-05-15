/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package priorities

import (
	"fmt"

	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	"k8s.io/kubernetes/pkg/scheduler/schedulercache"
)

// FunctionShape represents shape of scoring function.
// For safety use newFunctionShape which performs precondition checks for struct creation.
type FunctionShape struct {
	x []float64
	y []float64
	n int
}

var (
	// give priority to least utilized nodes by default
	defaultFunctionShape, _ = newFunctionShape([]float64{0.0, 1.0}, []float64{1.0, 0.0})
)

func newFunctionShape(x []float64, y []float64) (FunctionShape, error) {
	if len(x) != len(y) {
		return FunctionShape{}, fmt.Errorf("Length of x(%d) does not match length of y(%d)", len(x), len(y))
	}

	n := len(x)

	for i := 1; i < n; i++ {
		if x[i-1] >= x[i] {
			return FunctionShape{}, fmt.Errorf("Values in x must be sorted. x[%d]==%f >= x[%d]==%f", i-1, x[i-1], i, x[i])
		}
	}
	return FunctionShape{
		x: x,
		y: y,
		n: n,
	}, nil
}

// RequestedToCapacityRatioResourceAllocationPriorityDefault creates a requestedToCapacity based
// ResourceAllocationPriority using default resource scoring function shape.
// The default function assigns 1.0 to resource when all capacity is available
// and 0.0 when requested amount is equal to capacity.
func RequestedToCapacityRatioResourceAllocationPriorityDefault() *ResourceAllocationPriority {
	return RequestedToCapacityRatioResourceAllocationPriority(defaultFunctionShape)
}

// RequestedToCapacityRatioResourceAllocationPriority creates a requestedToCapacity based
// ResourceAllocationPriority using provided resource scoring function shape.
func RequestedToCapacityRatioResourceAllocationPriority(scoringFunctionShape FunctionShape) *ResourceAllocationPriority {
	return &ResourceAllocationPriority{"RequestedToCapacityRatioResourceAllocationPriority", buildRequestedToCapacityRatioScorerFunction(scoringFunctionShape)}
}

func buildRequestedToCapacityRatioScorerFunction(scoringFunctionShape FunctionShape) func(*schedulercache.Resource, *schedulercache.Resource, bool, int, int) int64 {
	rawScoringFunction := buildBrokenLinearFunction(scoringFunctionShape)

	resourceScoringFunction := func(requested, capacity int64) float64 {
		if capacity == 0 || requested > capacity {
			return rawScoringFunction(1.0)
		}

		return rawScoringFunction(1.0 - float64(capacity-requested)/float64(capacity))
	}

	return func(requested, allocable *schedulercache.Resource, includeVolumes bool, requestedVolumes int, allocatableVolumes int) int64 {
		cpuScore := resourceScoringFunction(requested.MilliCPU, allocable.MilliCPU)
		memoryScore := resourceScoringFunction(requested.Memory, allocable.Memory)
		return int64((cpuScore + memoryScore) / 2 * schedulerapi.MaxPriority)
	}
}

// Creates a function which is built using linear segments
// shape.x slice represents points on x axis where different segments meet
// shape.y represents function values at meeting points
// both shape.x and shape.y have length of shape.n
//
// function f(p) is defined as:
//   y[0] for p < x[0]
//   y[i] for p == x[i]
//   y[n-1] for p > x[n-1]
// and linear between points (p < x[i])
func buildBrokenLinearFunction(shape FunctionShape) func(float64) float64 {
	n := shape.n
	x := make([]float64, shape.n)
	copy(x, shape.x)
	y := make([]float64, shape.n)
	copy(y, shape.y)

	return func(p float64) float64 {
		for i := 0; i < n; i++ {
			if p <= x[i] {
				if i == 0 {
					return y[0]
				}
				return y[i-1] + (y[i]-y[i-1])*(p-x[i-1])/(x[i]-x[i-1])
			}
		}
		return y[n-1]
	}
}
