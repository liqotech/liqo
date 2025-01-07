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

package dynamic

import (
	"context"
	"encoding/json"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// CreateOrPatch creates or patches the object using the dynamic client.
func CreateOrPatch(ctx context.Context, c dynamic.ResourceInterface, objName string,
	mutate func(obj *unstructured.Unstructured) error) (*unstructured.Unstructured, error) {
	oldObj, err := c.Get(ctx, objName, metav1.GetOptions{})
	switch {
	case err == nil:
		// the object already exists, patch it
		newObj := oldObj.DeepCopy()
		if err := mutate(newObj); err != nil {
			return nil, fmt.Errorf("unable to patch the object: %w", err)
		}
		oldRaw, err := json.Marshal(oldObj)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal the old object: %w", err)
		}
		newRaw, err := json.Marshal(newObj)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal the new object: %w", err)
		}
		patch, err := jsonpatch.CreatePatch(oldRaw, newRaw)
		if err != nil {
			return nil, fmt.Errorf("unable to create the patch: %w", err)
		}
		if len(patch) == 0 {
			// nothing to patch
			return newObj, nil
		}
		patchRaw, err := json.Marshal(patch)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal the patch: %w", err)
		}
		return c.Patch(ctx, newObj.GetName(), types.JSONPatchType, patchRaw, metav1.PatchOptions{})
	case apierrors.IsNotFound(err):
		// the object does not exist, create it
		newObj := &unstructured.Unstructured{}
		if err := mutate(newObj); err != nil {
			return nil, fmt.Errorf("unable to create the object: %w", err)
		}
		return c.Create(ctx, newObj, metav1.CreateOptions{})
	default:
		return nil, fmt.Errorf("unable to get the object: %w", err)
	}
}
