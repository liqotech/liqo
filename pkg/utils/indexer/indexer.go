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

package indexer

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// FieldNodeNameFromPod is the field name of the node name of a pod.
	FieldNodeNameFromPod = "spec.nodeName"
)

// ExtractNodeName returns the node name of the given object.
func ExtractNodeName(rawObj client.Object) []string {
	switch obj := rawObj.(type) {
	case *corev1.Pod:
		return []string{obj.Spec.NodeName}
	default:
		return []string{}
	}
}

// IndexField indexes the given field on the given object.
func IndexField(ctx context.Context, mgr ctrlruntime.Manager, obj client.Object, field string, indexerFunc client.IndexerFunc) error {
	return mgr.GetFieldIndexer().IndexField(ctx, obj, field, indexerFunc)
}
