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
	"maps"
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

// createRemoteObject creates the remote twin of a local custom resource.
func createRemoteObject(ctx context.Context, client dynamic.ResourceInterface,
	local *unstructured.Unstructured, remoteNamespace string, forgingOpts *forge.ForgingOpts) (*unstructured.Unstructured, error) {
	remote, err := RemoteCustomResource(local, remoteNamespace, forgingOpts)
	if err != nil {
		return nil, err
	}
	return client.Create(ctx, remote, metav1.CreateOptions{})
}

// updateRemoteObjectSpec updates remote metadata and spec from the local object.
// It skips the API call when nothing changed.
func updateRemoteObjectSpec(ctx context.Context, client dynamic.ResourceInterface,
	local, remote *unstructured.Unstructured, forgingOpts *forge.ForgingOpts) (*unstructured.Unstructured, error) {
	oldLabels := maps.Clone(remote.GetLabels())
	oldAnnotations := maps.Clone(remote.GetAnnotations())
	ApplyRemoteMetadata(local, remote, forgingOpts)

	specLocal, err := getNestedMap(local, specKey)
	if err != nil {
		return remote, err
	}
	specRemote, err := getNestedMap(remote, specKey)
	if err != nil {
		return remote, err
	}

	specChanged := !reflect.DeepEqual(specLocal, specRemote)
	metaChanged := !reflect.DeepEqual(oldLabels, remote.GetLabels()) ||
		!reflect.DeepEqual(oldAnnotations, remote.GetAnnotations())

	if !specChanged && !metaChanged {
		return remote, nil
	}

	if specChanged {
		if err = unstructured.SetNestedMap(remote.Object, specLocal, specKey); err != nil {
			return remote, err
		}
	}

	return client.Update(ctx, remote, metav1.UpdateOptions{})
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
