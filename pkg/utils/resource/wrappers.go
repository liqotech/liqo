// Copyright 2019-2025 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CreateOrUpdate is a wrapper around controllerutil.CreateOrUpdate that automatically adds global labels and annotations.
// It takes the same parameters as controllerutil.CreateOrUpdate plus an optional list of additional labels to merge.
func CreateOrUpdate(ctx context.Context, c client.Client, obj client.Object,
	mutateFn func() error) (controllerutil.OperationResult, error) {
	// Create a wrapper mutation function that adds labels before calling the original
	wrappedMutateFn := func() error {
		// First call the original mutation function
		if err := mutateFn(); err != nil {
			return err
		}

		// Then add global labels and any additional labels
		if obj.GetLabels() == nil {
			obj.SetLabels(make(map[string]string))
		}
		labels := obj.GetLabels()
		for k, v := range GetGlobalLabels() {
			labels[k] = v
		}
		obj.SetLabels(labels)

		// Add global annotations
		if obj.GetAnnotations() == nil {
			obj.SetAnnotations(make(map[string]string))
		}
		annotations := obj.GetAnnotations()
		for k, v := range GetGlobalAnnotations() {
			annotations[k] = v
		}
		obj.SetAnnotations(annotations)

		return nil
	}

	return controllerutil.CreateOrUpdate(ctx, c, obj, wrappedMutateFn)
}
