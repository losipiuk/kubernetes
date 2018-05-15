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
	"fmt"
)

// Creates a function which is build using linear segments
// x slice represents points on x axis where different segments meet
// y represent function valus at meeting points
// len(x) must be equal to len(y).
//
// assuming n is len(x) function f(p) is defined as:
// y[0] for p < x[0]
// y[i] for p == x[i]
// y[n-1] for p > x[n-1]
// and linear between points
func brokenLinearFunction(x []float64, y [] float64) func(float64) float64 {
	if len(x) != len(y) {
		panic(fmt.Sprintf("Length of x(%d) does not match length of y(%d)", len(x), len(y)))
	}

	n := len(x)

	for i := 1; i < n; i++ {
		if x[i-1] >= x[i] {
			panic(fmt.Sprintf("Values in x must be increasing. x[%d]==%f >= x[%d]==%f", i-1, x[i-1], i, x[i]))
		}
	}

	xx := make([]float64, n)
	copy(xx, x)
	yy := make([]float64, n)
	copy(yy, y)

	return func(p float64) float64 {
		for i := 0; i < n; i++ {
			if p <= xx[i] {
				if i == 0 {
					return yy[0]
				} else {
					return yy[i-1] + (yy[i]-yy[i-1])*(xx[i]-xx[i-1])/(p-xx[i-1])
				}
			}
		}
		return yy[n-1]
	}
}
