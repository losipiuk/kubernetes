/*
Copyright 2016 The Kubernetes Authors.

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
	"testing"
)

func TestCreatingBrokenLinearFunctionPanicsIfLengthOfXDoesNotMatchLengthOfY(t *testing.T) {
	assert.PanicsWithValue(
		t,
		"Length of x(2) does not match length of y(3)",
		func() { brokenLinearFunction([]float64{1, 2}, []float64{1, 2, 3}) })
}

func TestCreatingBrokenLinearFunctionPanicsIfXIsNotIncreasing(t *testing.T) {
	assert.PanicsWithValue(
		t,
		"Values in x must be increasing. x[2]==2.000000 >= x[3]==1.900000",
		func() { brokenLinearFunction([]float64{1, 1.5, 2, 1.9, 2.5}, []float64{1, 2, 3, 4, 5}) })
	assert.PanicsWithValue(
		t,
		"Values in x must be increasing. x[1]==2.000000 >= x[2]==2.000000",
		func() { brokenLinearFunction([]float64{1, 2, 2, 2.2, 2.5}, []float64{1, 2, 3, 4, 5}) })
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
	}

	for _, test := range tests {
		function := brokenLinearFunction(test.x, test.y)
		for _, assertion := range test.assertions {
			assert.InDelta(t, assertion.expected, function(assertion.p), 0.1, "x=%v, y=%v, p=%f", test.x, test.y, assertion.p)
		}
	}

}
