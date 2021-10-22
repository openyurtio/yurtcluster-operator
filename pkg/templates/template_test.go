/*
Copyright 2021 The OpenYurt Authors.

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

package templates

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestMergeExtraArgsForContainer(t *testing.T) {
	var tests = []struct {
		name      string
		container *corev1.Container
		extraArgs map[string]string
		expected  *corev1.Container
	}{
		{
			name: "append extra args",
			container: &corev1.Container{
				Args: []string{
					"--foo=bar",
				},
			},
			extraArgs: map[string]string{
				"--a": "b",
				"c":   "d",
			},
			expected: &corev1.Container{
				Args: []string{
					"--a=b",
					"--c=d",
					"--foo=bar",
				},
			},
		},
		{
			name: "replace current args",
			container: &corev1.Container{
				Args: []string{
					"--only-flag",
					"--foo=bar",
				},
			},
			extraArgs: map[string]string{
				"foo": "car",
				"c":   "d",
			},
			expected: &corev1.Container{
				Args: []string{
					"--c=d",
					"--foo=car",
					"--only-flag",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			MergeExtraArgsForContainer(tc.container, tc.extraArgs)
			if !reflect.DeepEqual(tc.container, tc.expected) {
				t.Errorf("expect command %v, but got %v", tc.expected.Command, tc.container.Command)
			}
		})
	}
}
