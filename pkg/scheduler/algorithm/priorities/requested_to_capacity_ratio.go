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
// For safety use NewFunctionShape which performs precondition checks for struct creation.
type FunctionShape []FunctionShapePoint

// FunctionShapePoint represents single point in scoring function shape.
type FunctionShapePoint struct {
	// X is function argument.
	X int64
	// Y is function value.
	Y int64
}

var (
	// give priority to least utilized nodes by default
	defaultFunctionShape, _ = NewFunctionShape([]FunctionShapePoint{{0, 10}, {100, 0}})
)

const (
	minX = 0
	maxX = 100
	minY = 0
	maxY = schedulerapi.MaxPriority
)

// NewFunctionShape creates instance of FunctionShape in a safe way performing all
// necessary sanity checks.
func NewFunctionShape(points []FunctionShapePoint) (FunctionShape, error) {

	n := len(points)

	if n == 0 {
		return nil, fmt.Errorf("at least one point must be specified")
	}

	for i := 1; i < n; i++ {
		if points[i-1].X >= points[i].X {
			return nil, fmt.Errorf("values in x must be sorted. X[%d]==%d >= X[%d]==%d", i-1, points[i-1].X, i, points[i].X)
		}
	}

	for i, point := range points {
		if point.X < minX {
			return nil, fmt.Errorf("values in x must not be less than %d. X[%d]==%d", minX, i, point.X)
		}
		if point.X > maxX {
			return nil, fmt.Errorf("values in x must not be greater than %d. X[%d]==%d", maxX, i, point.X)
		}
		if point.Y < minY {
			return nil, fmt.Errorf("values in y must not be less than %d. y[%d]==%d", minY, i, point.Y)
		}
		if point.Y > maxY {
			return nil, fmt.Errorf("values in y must not be greater than %d. y[%d]==%d", maxY, i, point.Y)
		}
	}

	// We make defensive copy so we make no assumption if array passed as argument is not changed afterwards
	pointsCopy := make(FunctionShape, n)
	copy(pointsCopy, points)
	return pointsCopy, nil
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

	resourceScoringFunction := func(requested, capacity int64) int64 {
		if capacity == 0 || requested > capacity {
			return rawScoringFunction(maxX)
		}

		return rawScoringFunction(maxX - (capacity-requested)*maxX/capacity)
	}

	return func(requested, allocable *schedulercache.Resource, includeVolumes bool, requestedVolumes int, allocatableVolumes int) int64 {
		cpuScore := resourceScoringFunction(requested.MilliCPU, allocable.MilliCPU)
		memoryScore := resourceScoringFunction(requested.Memory, allocable.Memory)
		return (cpuScore + memoryScore) / 2
	}
}

// Creates a function which is built using linear segments
// shape.x slice represents points on X axis where different segments meet
// shape.y represents function values at meeting points
//
// function f(p) is defined as:
//   y[0] for p < x[0]
//   y[i] for p == x[i]
//   y[n-1] for p > x[n-1]
// and linear between points (p < x[i])
func buildBrokenLinearFunction(shape FunctionShape) func(int64) int64 {
	n := len(shape)
	return func(p int64) int64 {
		for i := 0; i < n; i++ {
			if p <= shape[i].X {
				if i == 0 {
					return shape[0].Y
				}
				return shape[i-1].Y + (shape[i].Y-shape[i-1].Y)*(p-shape[i-1].X)/(shape[i].X-shape[i-1].X)
			}
		}
		return shape[n-1].Y
	}
}
