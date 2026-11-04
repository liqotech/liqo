// Copyright 2019-2026 The Liqo Authors
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

package custom

import (
	"context"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const (
	specKey   = "spec"
	statusKey = "status"
)

// getNestedMap retrieves a nested map from an unstructured object, returning an empty map if not found.
func getNestedMap(unstr *unstructured.Unstructured, key string) (map[string]interface{}, error) {
	nested, found, err := unstructured.NestedMap(unstr.Object, key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %v key: %w", key, err)
	}
	if !found {
		nested = map[string]interface{}{}
	}
	return nested, nil
}

// applyRemoteObject creates or updates the remote twin of a local custom resource
// through server-side apply, mirroring the approach of the built-in reflectors:
// the reflector owns the spec and the reflected metadata, while fields owned by
// other field managers (e.g. annotations added on the remote twin by other
// controllers) are preserved. The remote status is not part of the applied
// configuration, hence it is never overwritten.
func applyRemoteObject(ctx context.Context, client dynamic.ResourceInterface,
	local *unstructured.Unstructured, remoteNamespace string, forgingOpts *forge.ForgingOpts) (*unstructured.Unstructured, error) {
	remote, err := RemoteCustomResource(local, remoteNamespace, forgingOpts)
	if err != nil {
		return nil, err
	}
	return client.Apply(ctx, local.GetName(), remote, forge.ApplyOptions())
}

// updateObjectStatusShared syncs status from source (remote) to destination (local) — OwnershipShared direction.
func updateObjectStatusShared(ctx context.Context, localClient dynamic.ResourceInterface,
	gvr schema.GroupVersionResource, source, destination *unstructured.Unstructured) error {
	statusSource, found, err := unstructured.NestedMap(source.Object, statusKey)
	if err != nil {
		return fmt.Errorf("failed to retrieve %v key: %w", statusKey, err)
	}
	if !found {
		return nil
	}
	statusDestination, err := getNestedMap(destination, statusKey)
	if err != nil {
		return err
	}

	if reflect.DeepEqual(statusSource, statusDestination) {
		return nil
	}

	if err = unstructured.SetNestedMap(destination.Object, statusSource, statusKey); err != nil {
		return err
	}

	_, err = localClient.UpdateStatus(ctx, destination, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(4).Infof("Status of %v %q successfully updated from remote", gvr, destination.GetName())
	return nil
}
