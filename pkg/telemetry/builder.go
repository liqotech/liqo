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

package telemetry

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
)

// ForgeTelemetryItem returns a Telemetry item with the current status of the cluster.
func (c *Builder) ForgeTelemetryItem(ctx context.Context) (*Telemetry, error) {
	clusterID, err := utils.GetClusterIDWithControllerClient(ctx, c.Client, c.Namespace)
	if err != nil {
		return nil, err
	}

	return &Telemetry{
		ClusterID:         string(clusterID),
		LiqoVersion:       c.LiqoVersion,
		KubernetesVersion: c.KubernetesVersion,
		NodesInfo:         c.getNodesInfo(ctx),
		Provider:          c.getProvider(),
		PeeringInfo:       c.getPeeringInfoSlice(ctx),
		NamespacesInfo:    c.getNamespacesInfo(ctx),
	}, nil
}

func (c *Builder) getNodesInfo(ctx context.Context) map[string]NodeInfo {
	nodes, err := liqogetters.ListNotLiqoNodes(ctx, c.Client)
	runtime.Must(err)

	nodeInfoMap := make(map[string]NodeInfo, len(nodes.Items))
	for i := range nodes.Items {
		node := &nodes.Items[i]
		nodeInfoMap[node.Name] = NodeInfo{
			KernelVersion: node.Status.NodeInfo.KernelVersion,
			OsImage:       node.Status.NodeInfo.OSImage,
			Architecture:  node.Status.NodeInfo.Architecture,
		}
	}
	return nodeInfoMap
}

func (c *Builder) getProvider() string {
	return c.ClusterLabels[consts.ProviderClusterLabel]
}

func (c *Builder) getNamespacesInfo(ctx context.Context) []NamespaceInfo {
	var namespaceOffloadings offloadingv1beta1.NamespaceOffloadingList
	err := c.Client.List(ctx, &namespaceOffloadings)
	runtime.Must(err)

	virtualNodes, err := liqogetters.ListVirtualNodesByLabels(ctx, c.Client, labels.Everything())
	runtime.Must(err)

	nodeNameClusterIDMap := map[string]string{}
	for i := range virtualNodes.Items {
		virtualNode := &virtualNodes.Items[i]
		clusterID := virtualNode.Spec.ClusterID
		nodeNameClusterIDMap[virtualNode.Name] = string(clusterID)
	}

	namespaceInfoSlice := make([]NamespaceInfo, len(namespaceOffloadings.Items))
	for i := range namespaceOffloadings.Items {
		namespaceOffloading := &namespaceOffloadings.Items[i]
		namespaceInfoSlice[i] = c.getNamespaceInfo(ctx, namespaceOffloading, nodeNameClusterIDMap)
	}
	return namespaceInfoSlice
}

func (c *Builder) getNamespaceInfo(ctx context.Context,
	namespaceOffloading *offloadingv1beta1.NamespaceOffloading, nodeNameClusterIDMap map[string]string) NamespaceInfo {
	namespaceInfo := NamespaceInfo{
		UID:                string(namespaceOffloading.GetUID()),
		MappingStrategy:    namespaceOffloading.Spec.NamespaceMappingStrategy,
		OffloadingStrategy: namespaceOffloading.Spec.PodOffloadingStrategy,
		HasClusterSelector: len(namespaceOffloading.Spec.ClusterSelector.NodeSelectorTerms) > 0,
		NumOffloadedPods:   map[string]int64{},
	}

	offloadedPods, err := liqogetters.ListOffloadedPods(ctx, c.Client, namespaceOffloading.Namespace)
	runtime.Must(err)

	var nodePodBucket = make(map[string]int64)
	for i := range offloadedPods.Items {
		pod := &offloadedPods.Items[i]
		if pod.Spec.NodeName == "" {
			klog.Warningf("Pod %s has no node assigned", klog.KObj(pod))
			continue
		}

		nodePodBucket[pod.Spec.NodeName]++
	}

	for nodeName, clusterID := range nodeNameClusterIDMap {
		namespaceInfo.NumOffloadedPods[clusterID] = nodePodBucket[nodeName]
	}
	return namespaceInfo
}

func (c *Builder) getPeeringInfoSlice(ctx context.Context) []PeeringInfo {
	var foreignClusterList liqov1beta1.ForeignClusterList
	err := c.Client.List(ctx, &foreignClusterList)
	runtime.Must(err)

	peeringInfoSlice := make([]PeeringInfo, len(foreignClusterList.Items))
	for i := range foreignClusterList.Items {
		foreignCluster := &foreignClusterList.Items[i]
		peeringInfoSlice[i] = c.getPeeringInfo(foreignCluster)
	}
	return peeringInfoSlice
}

func getNodesNumber(cl client.Client, fc *liqov1beta1.ForeignCluster) int {
	var nodesNumber int
	nodeList, err := liqogetters.ListNodesByClusterID(context.Background(), cl, fc.Spec.ClusterID)
	runtime.Must(client.IgnoreNotFound(err))
	if nodeList == nil {
		nodesNumber = 0
	} else {
		nodesNumber = len(nodeList.Items)
	}
	return nodesNumber
}

func getVirtualNodesNumber(cl client.Client, fc *liqov1beta1.ForeignCluster) int {
	virtualNodes, err := liqogetters.ListVirtualNodesByClusterID(context.Background(), cl, fc.Spec.ClusterID)
	runtime.Must(err)
	return len(virtualNodes)
}

func getResourceSliceNumber(cl client.Client, fc *liqov1beta1.ForeignCluster) int {
	resSlicesList, err := liqogetters.ListResourceSlicesByLabel(context.Background(), cl, corev1.NamespaceAll,
		liqolabels.LocalLabelSelectorForCluster(string(fc.Spec.ClusterID)))
	runtime.Must(err)
	return len(resSlicesList)
}

func (c *Builder) getPeeringInfo(foreignCluster *liqov1beta1.ForeignCluster) PeeringInfo {
	var latency time.Duration

	peeringInfo := PeeringInfo{
		RemoteClusterID: foreignCluster.Spec.ClusterID,
		Modules: ModulesInfo{
			Networking:     ModuleInfo{Enabled: foreignCluster.Status.Modules.Networking.Enabled},
			Authentication: ModuleInfo{Enabled: foreignCluster.Status.Modules.Authentication.Enabled},
			Offloading:     ModuleInfo{Enabled: foreignCluster.Status.Modules.Offloading.Enabled},
		},
		Role:                foreignCluster.Status.Role,
		Latency:             latency,
		NodesNumber:         getNodesNumber(c.Client, foreignCluster),
		VirtualNodesNumber:  getVirtualNodesNumber(c.Client, foreignCluster),
		ResourceSliceNumber: getResourceSliceNumber(c.Client, foreignCluster),
	}
	return peeringInfo
}
