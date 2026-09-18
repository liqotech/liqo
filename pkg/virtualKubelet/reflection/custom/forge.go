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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// RemoteCustomResource forges a remote unstructured object from the local one.
// Only metadata (filtered labels/annotations) and the spec are copied.
// OwnerReferences are intentionally not propagated across clusters.
func RemoteCustomResource(local *unstructured.Unstructured, targetNamespace string,
	forgingOpts *forge.ForgingOpts) (*unstructured.Unstructured, error) {
	remote := &unstructured.Unstructured{}
	remote.SetGroupVersionKind(local.GroupVersionKind())
	remote.SetNamespace(targetNamespace)
	remote.SetName(local.GetName())

	labelsNotReflected, annotationsNotReflected := notReflectedKeys(forgingOpts)
	filteredLabels := forge.FilterNotReflected(local.GetLabels(), labelsNotReflected)
	remote.SetLabels(labels.Merge(filteredLabels, forge.ReflectionLabels()))
	remote.SetAnnotations(forge.FilterNotReflected(local.GetAnnotations(), annotationsNotReflected))

	spec, found, err := unstructured.NestedMap(local.Object, specKey)
	if err != nil {
		return nil, fmt.Errorf("invalid spec on %s/%s: %w", local.GetNamespace(), local.GetName(), err)
	}
	if found {
		if err := unstructured.SetNestedMap(remote.Object, spec, specKey); err != nil {
			return nil, err
		}
	}

	return remote, nil
}

// ApplyRemoteMetadata updates labels and annotations on an existing remote object from the local one.
func ApplyRemoteMetadata(local, remote *unstructured.Unstructured, forgingOpts *forge.ForgingOpts) {
	labelsNotReflected, annotationsNotReflected := notReflectedKeys(forgingOpts)
	filteredLabels := forge.FilterNotReflected(local.GetLabels(), labelsNotReflected)
	remote.SetLabels(labels.Merge(filteredLabels, forge.ReflectionLabels()))
	remote.SetAnnotations(forge.FilterNotReflected(local.GetAnnotations(), annotationsNotReflected))
}

func notReflectedKeys(forgingOpts *forge.ForgingOpts) (labelsNotReflected, annotationsNotReflected []string) {
	if forgingOpts == nil {
		return nil, nil
	}
	return forgingOpts.LabelsNotReflected, forgingOpts.AnnotationsNotReflected
}
