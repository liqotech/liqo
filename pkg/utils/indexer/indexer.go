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

package indexer

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	ctrlruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/directconnection"
)

const (
	// FieldNodeNameFromPod is the field name of the node name of a pod.
	FieldNodeNameFromPod = "spec.nodeName"

	// FieldDirectConnectionClusterIDs is the field index key used to index ShadowEndpointSlices by the
	// clusterIDs present as keys in the direct-connections-data annotation (i.e., the clusterAddresses map).
	FieldDirectConnectionClusterIDs = "metadata.annotations.direct-connections-data.clusterIDs"
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

// ExtractDirectConnectionClusterIDs returns all clusterIDs that appear as keys in the
// direct-connections-data annotation of a ShadowEndpointSlice. controller-runtime uses
// the returned slice to build a multi-value field index, so a single object can be found
// by any of its clusterIDs via client.MatchingFields.
func ExtractDirectConnectionClusterIDs(rawObj client.Object) []string {
	shadow, ok := rawObj.(*offloadingv1beta1.ShadowEndpointSlice)
	if !ok {
		return nil
	}
	val, ok := shadow.Annotations[consts.DirectConnectionDataAnnotationKey]
	if !ok {
		return nil
	}
	var ca directconnection.ClusterAddresses
	if err := ca.FromJSON([]byte(val)); err != nil {
		return nil
	}
	clusterIDs := make([]string, 0, len(ca.Clusters))
	for id := range ca.Clusters {
		clusterIDs = append(clusterIDs, id)
	}
	return clusterIDs
}

// IndexField indexes the given field on the given object.
func IndexField(ctx context.Context, mgr ctrlruntime.Manager, obj client.Object, field string, indexerFunc client.IndexerFunc) error {
	return mgr.GetFieldIndexer().IndexField(ctx, obj, field, indexerFunc)
}
