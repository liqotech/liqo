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

package remotemetrics

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	liqoutils "github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

type resourceGetter struct {
	cl client.Client
}

// NewResourceGetter creates a new resource getter.
func NewResourceGetter(cl client.Client) ResourceGetter {
	return &resourceGetter{
		cl: cl,
	}
}

// GetNamespaces returns the names of all namespaces in the cluster owned by the remote clusterID.
func (m *resourceGetter) GetNamespaces(ctx context.Context, clusterID string) []MappedNamespace {
	namespaces := &corev1.NamespaceList{}

	originIDReq, err := labels.NewRequirement(consts.RemoteClusterID, selection.Equals, []string{clusterID})
	utilruntime.Must(err)

	err = m.cl.List(ctx, namespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(*originIDReq),
	})
	utilruntime.Must(err)

	res := []MappedNamespace{}
	for i := range namespaces.Items {
		namespace := &namespaces.Items[i]

		if namespace.Annotations == nil || namespace.Annotations[consts.RemoteNamespaceOriginalNameAnnotationKey] == "" {
			continue
		}

		res = append(res, MappedNamespace{
			Namespace:    namespace.GetName(),
			OriginalName: namespace.Annotations[consts.RemoteNamespaceOriginalNameAnnotationKey],
		})
	}

	klog.V(2).Infof("Scraping namespaces %+v for cluster id %s", res, clusterID)
	return res
}

// GetPodNames returns the names of all pods in the cluster owned by the remote clusterID and scheduled in the given node.
func (m *resourceGetter) GetPodNames(ctx context.Context, clusterID, node string) []string {
	pods := &corev1.PodList{}

	clIDReq, err := labels.NewRequirement(forge.LiqoOriginClusterIDKey, selection.Equals, []string{clusterID})
	utilruntime.Must(err)

	err = m.cl.List(ctx, pods, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(*clIDReq),
	})
	utilruntime.Must(err)

	res := []string{}
	for i := range pods.Items {
		pod := &pods.Items[i]

		if pod.Spec.NodeName == node {
			res = append(res, pod.Name)
		}
	}

	klog.V(2).Infof("Scraping pods %+v for cluster id %s and node %s", res, clusterID, node)
	return res
}

// GetNodeNames returns the names of all physical nodes in the cluster.
func (m *resourceGetter) GetNodeNames(ctx context.Context) []string {
	nodes := &corev1.NodeList{}

	// we exclude virtual nodes to avoid infinite loops, both for bidirectional peerings
	// and for cycling peerings A -> B -> C -> A that would lead to non-terminating HTTP requests.
	realNode, err := labels.NewRequirement(consts.TypeLabel, selection.NotIn, []string{consts.TypeNode})
	utilruntime.Must(err)

	err = m.cl.List(ctx, nodes, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(*realNode),
	})
	utilruntime.Must(err)

	res := make([]string, 0)
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if liqoutils.IsNodeReady(node) {
			res = append(res, node.Name)
		}
	}

	klog.V(2).Infof("Scraping nodes %+v", res)
	return res
}
