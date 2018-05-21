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
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	"k8s.io/kubernetes/pkg/scheduler/schedulercache"
	"sort"
	"k8s.io/kubernetes/bazel-kubernetes/external/go_sdk/src/fmt"
)

func TestCreatingFunctionShapePanicsIfLengthOfXDoesNotMatchLengthOfY(t *testing.T) {
	assert.PanicsWithValue(
		t,
		"Length of x(2) does not match length of y(3)",
		func() { newFunctionShape([]float64{1, 2}, []float64{1, 2, 3}) })
}

func TestCreatingFunctionShapePanicsIfXIsNotIncreasing(t *testing.T) {
	assert.PanicsWithValue(
		t,
		"Values in x must be increasing. x[2]==2.000000 >= x[3]==1.900000",
		func() { newFunctionShape([]float64{1, 1.5, 2, 1.9, 2.5}, []float64{1, 2, 3, 4, 5}) })
	assert.PanicsWithValue(
		t,
		"Values in x must be increasing. x[1]==2.000000 >= x[2]==2.000000",
		func() { newFunctionShape([]float64{1, 2, 2, 2.2, 2.5}, []float64{1, 2, 3, 4, 5}) })
}

func TestBrokenLinearFunction(t *testing.T) {
	type Assertion struct {
		p        float64
		expected float64
	}
	type Test struct {
		x          []float64
		y          []float64
		assertions []Assertion
	}

	tests := []Test{
		{
			x: []float64{100.0, 200.0},
			y: []float64{2000.0, 3000.0},
			assertions: []Assertion{
				{p: -500.0, expected: 2000.0},
				{p: 0.0, expected: 2000.0},
				{p: 99.0, expected: 2000.0},
				{p: 100.0, expected: 2000.0},
				{p: 101.0, expected: 2010.0},
				{p: 199.0, expected: 2990.0},
				{p: 199.0, expected: 2990.0},
				{p: 200.0, expected: 3000.0},
				{p: 201.0, expected: 3000.0},
				{p: 500.0, expected: 3000.0},
			},
		},
		{
			x: []float64{0.0, 40.0, 100.0},
			y: []float64{2.0, 10.0, 0.0},
			assertions: []Assertion{
				{p: -10.0, expected: 2.0},
				{p: 0.0, expected: 2.0},
				{p: 20.0, expected: 6.0},
				{p: 30.0, expected: 8.0},
				{p: 40.0, expected: 10.0},
				{p: 70.0, expected: 5.0},
				{p: 100.0, expected: 0.0},
				{p: 110.0, expected: 0.0},
			},
		},
		{
			x: []float64{0.0, 40.0, 100.0},
			y: []float64{2.0, 2.0, 2.0},
			assertions: []Assertion{
				{p: -10.0, expected: 2.0},
				{p: 0.0, expected: 2.0},
				{p: 20.0, expected: 2.0},
				{p: 30.0, expected: 2.0},
				{p: 40.0, expected: 2.0},
				{p: 70.0, expected: 2.0},
				{p: 100.0, expected: 2.0},
				{p: 110.0, expected: 2.0},
			},
		},
	}

	for _, test := range tests {
		function := buildBrokenLinearFunction(newFunctionShape(test.x, test.y))
		for _, assertion := range test.assertions {
			assert.InDelta(t, assertion.expected, function(assertion.p), 0.1, "x=%v, y=%v, p=%f", test.x, test.y, assertion.p)
		}
	}
}

func TestRequestedToCapacityRatio(t *testing.T) {
	type resources struct {
		cpu int64
		mem int64
	}

	type nodeResources struct {
		capacity resources
		used     resources
	}

	type test struct {
		test               string
		requested          resources
		nodes              map[string]nodeResources
		expectedPriorities schedulerapi.HostPriorityList
	}

	tests := []test{
		{
			test:      "nothing scheduled, nothing requested (default - most requested nodes have priority)",
			requested: resources{0, 0},
			nodes: map[string]nodeResources{
				"node1": {
					capacity: resources{4000, 10000},
					used:     resources{0, 0},
				},
				"node2": {
					capacity: resources{4000, 10000},
					used:     resources{0, 0},
				},
			},
			expectedPriorities: []schedulerapi.HostPriority{{Host: "node1", Score: 0}, {Host: "node2", Score: 0}},
		},
		{
			test:      "nothing scheduled, resources requested, differently sized machines (default - most requested nodes have priority)",
			requested: resources{3000, 5000},
			nodes: map[string]nodeResources{
				"node1": {
					capacity: resources{4000, 10000},
					used:     resources{0, 0},
				},
				"node2": {
					capacity: resources{6000, 10000},
					used:     resources{0, 0},
				},
			},
			expectedPriorities: []schedulerapi.HostPriority{{Host: "node1", Score: 6}, {Host: "node2", Score: 5}},
		},
		{
			test:      "no resources requested, pods scheduled with resources (default - most requested nodes have priority)",
			requested: resources{0, 0},
			nodes: map[string]nodeResources{
				"node1": {
					capacity: resources{4000, 10000},
					used:     resources{3000, 5000},
				},
				"node2": {
					capacity: resources{6000, 10000},
					used:     resources{3000, 5000},
				},
			},
			expectedPriorities: []schedulerapi.HostPriority{{Host: "node1", Score: 6}, {Host: "node2", Score: 5}},
		},
	}

	buildResourcesPod := func(node string, requestedResources resources) *v1.Pod {
		return &v1.Pod{Spec: v1.PodSpec{
			NodeName: node,
			Containers: []v1.Container{
				{
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    *resource.NewMilliQuantity(requestedResources.cpu, resource.DecimalSI),
							v1.ResourceMemory: *resource.NewQuantity(requestedResources.mem, resource.DecimalSI),
						},
					},
				},
			},
		},
		}
	}

	for _, test := range tests {

		nodeNames := make([]string, 0)
		for nodeName := range test.nodes {
			nodeNames = append(nodeNames, nodeName)
		}
		sort.Strings(nodeNames)

		nodes := make([]*v1.Node, 0)
		for _, nodeName := range nodeNames {
			node := test.nodes[nodeName]
			nodes = append(nodes, makeNode(nodeName, node.capacity.cpu, node.capacity.mem))
		}

		scheduledPods := make([]*v1.Pod, 0)
		for name, node := range test.nodes {
			scheduledPods = append(scheduledPods,
				buildResourcesPod(name, node.used))
		}

		newPod := buildResourcesPod("", test.requested)

		nodeNameToInfo := schedulercache.CreateNodeNameToInfoMap(scheduledPods, nodes)
		list, err := priorityFunction(RequestedToCapacityRatioResourceAllocationPriorityDefault().PriorityMap, nil, nil)(newPod, nodeNameToInfo, nodes)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(test.expectedPriorities, list) {
			t.Errorf("%s: expected %#v, got %#v", test.test, test.expectedPriorities, list)
		}
	}
}

func TestParseRequestedToCapacityRatioScoringFunctionShape(t *testing.T) {
	type test struct {
		shapeDesc     string
		expectedPanic bool
		expectedErr   string
		expectedX     []float64
		expectedY     []float64
	}
	tests := []test{
		{
			shapeDesc:     "0.0=0.1",
			expectedPanic: false,
			expectedX:     []float64{0.0},
			expectedY:     []float64{0.1},
		},
		{
			shapeDesc:     "0.0=0.1,0.3=0.4",
			expectedPanic: false,
			expectedX:     []float64{0.0, 0.3},
			expectedY:     []float64{0.1, 0.4},
		},
		{
			shapeDesc:     "0=0.1,0.3=0.4",
			expectedPanic: false,
			expectedX:     []float64{0.0, 0.3},
			expectedY:     []float64{0.1, 0.4},
		},
		{
			shapeDesc:     "",
			expectedPanic: true,
		},
		{
			shapeDesc:     "0.0=0.1,0.3=x",
			expectedPanic: true,
		},
		{
			shapeDesc:     "0.3=0.4,0.0=0.1",
			expectedErr:   "Values in x must be increasing. x[0]==0.300000 >= x[1]==0.000000",
			expectedPanic: true,
		},
		{
			shapeDesc:     "blah",
			expectedPanic: true,
		},
		{
			shapeDesc:     "0.0",
			expectedPanic: true,
		},
	}

	for _, test := range tests {
		if test.expectedPanic {
			expectedPanicMessage := fmt.Sprintf("Cannot parse function shape '%s'", test.shapeDesc)
			if len(test.expectedErr) != 0 {
				expectedPanicMessage = expectedPanicMessage + fmt.Sprintf("; err='%s'", test.expectedErr)
			}
			assert.PanicsWithValue(
				t,
				expectedPanicMessage,
				func() { ParseRequestedToCapacityRatioScoringFunctionShape(test.shapeDesc) })
		} else {
			expectedShape := newFunctionShape(test.expectedX, test.expectedY)
			parsedShape := ParseRequestedToCapacityRatioScoringFunctionShape(test.shapeDesc)
			assert.Equal(t, expectedShape, parsedShape)
		}
	}
}
