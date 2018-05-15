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
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
	"k8s.io/kubernetes/pkg/scheduler/schedulercache"
)

func TestCreatingFunctionShapePanicsIfLengthOfXDoesNotMatchLengthOfY(t *testing.T) {
	var err error
	_, err = NewFunctionShape([]int64{1, 2}, []int64{1, 2, 3})
	assert.Equal(t, "length of x(2) does not match length of y(3)", err.Error())
}

func TestCreatingFunctionShapeErrorsIfXIsNotSorted(t *testing.T) {
	var err error
	_, err = NewFunctionShape([]int64{10, 15, 20, 19, 25}, []int64{1, 2, 3, 4, 5})
	assert.Equal(t, "values in x must be sorted. x[2]==20 >= x[3]==19", err.Error())

	_, err = NewFunctionShape([]int64{10, 20, 20, 22, 25}, []int64{1, 2, 3, 4, 5})
	assert.Equal(t, "values in x must be sorted. x[1]==20 >= x[2]==20", err.Error())
}

func TestCreatingFunctionPointNotInAllowedRange(t *testing.T) {
	var err error
	_, err = NewFunctionShape([]int64{-1, 100}, []int64{0, 10})
	assert.Equal(t, "values in x must not be less than 0. x[0]==-1", err.Error())

	_, err = NewFunctionShape([]int64{0, 101}, []int64{0, 10})
	assert.Equal(t, "values in x must not be greater than 100. x[1]==101", err.Error())

	_, err = NewFunctionShape([]int64{0, 100}, []int64{-1, 10})
	assert.Equal(t, "values in y must not be less than 0. y[0]==-1", err.Error())

	_, err = NewFunctionShape([]int64{0, 100}, []int64{0, 11})
	assert.Equal(t, "values in y must not be greater than 10. y[1]==11", err.Error())
}

func TestBrokenLinearFunction(t *testing.T) {
	type Assertion struct {
		p        int64
		expected int64
	}
	type Test struct {
		x          []int64
		y          []int64
		assertions []Assertion
	}

	tests := []Test{
		{
			x: []int64{10, 90},
			y: []int64{1, 9},
			assertions: []Assertion{
				{p: -10, expected: 1},
				{p: 0, expected: 1},
				{p: 9, expected: 1},
				{p: 10, expected: 1},
				{p: 15, expected: 1},
				{p: 19, expected: 1},
				{p: 20, expected: 2},
				{p: 89, expected: 8},
				{p: 90, expected: 9},
				{p: 99, expected: 9},
				{p: 100, expected: 9},
				{p: 110, expected: 9},
			},
		},
		{
			x: []int64{0, 40, 100},
			y: []int64{2, 10, 0},
			assertions: []Assertion{
				{p: -10, expected: 2},
				{p: 0, expected: 2},
				{p: 20, expected: 6},
				{p: 30, expected: 8},
				{p: 40, expected: 10},
				{p: 70, expected: 5},
				{p: 100, expected: 0},
				{p: 110, expected: 0},
			},
		},
		{
			x: []int64{0, 40, 100},
			y: []int64{2, 2, 2},
			assertions: []Assertion{
				{p: -10, expected: 2},
				{p: 0, expected: 2},
				{p: 20, expected: 2},
				{p: 30, expected: 2},
				{p: 40, expected: 2},
				{p: 70, expected: 2},
				{p: 100, expected: 2},
				{p: 110, expected: 2},
			},
		},
	}

	for _, test := range tests {
		functionShape, err := NewFunctionShape(test.x, test.y)
		assert.Nil(t, err)
		function := buildBrokenLinearFunction(functionShape)
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
			test:      "nothing scheduled, nothing requested (default - least requested nodes have priority)",
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
			expectedPriorities: []schedulerapi.HostPriority{{Host: "node1", Score: 10}, {Host: "node2", Score: 10}},
		},
		{
			test:      "nothing scheduled, resources requested, differently sized machines (default - least requested nodes have priority)",
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
			expectedPriorities: []schedulerapi.HostPriority{{Host: "node1", Score: 4}, {Host: "node2", Score: 5}},
		},
		{
			test:      "no resources requested, pods scheduled with resources (default - least requested nodes have priority)",
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
			expectedPriorities: []schedulerapi.HostPriority{{Host: "node1", Score: 4}, {Host: "node2", Score: 5}},
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
