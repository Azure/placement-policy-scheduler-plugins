/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestSchedulingDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config runtime.Object
		expect runtime.Object
	}{}

	for _, tc := range tests {
		scheme := runtime.NewScheme()
		utilruntime.Must(AddToScheme(scheme))
		t.Run(tc.name, func(t *testing.T) {
			scheme.Default(tc.config)
			if diff := cmp.Diff(tc.config, tc.expect); diff != "" {
				t.Errorf("Got unexpected defaults (-want, +got):\n%s", diff)
			}
		})
	}
}
