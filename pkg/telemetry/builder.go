// Copyright 2019-2023 The Liqo Authors
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/discovery"
	"github.com/liqotech/liqo/pkg/utils"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
	labelsutils "github.com/liqotech/liqo/pkg/utils/labels"
	peeringconditionsutils "github.com/liqotech/liqo/pkg/utils/peeringConditions"
)

// ForgeTelemetryItem returns a Telemetry item with the current status of the cluster.
func (c *Builder) ForgeTelemetryItem(ctx context.Context) (*Telemetry, error) {
	clusterIdentity, err := utils.GetClusterIdentityWithControllerClient(ctx, c.Client, c.Namespace)
	if err != nil {
		return nil, err
	}

	return &Telemetry{
		ClusterID:         clusterIdentity.ClusterID,
		LiqoVersion:       c.LiqoVersion,
		KubernetesVersion: c.KubernetesVersion,
		Provider:          c.getProvider(),
		PeeringInfo:       c.getPeeringInfoSlice(ctx),
		NamespacesInfo:    c.getNamespacesInfo(ctx),
	}, nil
}

func (c *Builder) getProvider() string {
	return c.ClusterLabels[consts.ProviderClusterLabel]
}

func (c *Builder) getNamespacesInfo(ctx context.Context) []NamespaceInfo {
	var namespaceOffloadings offloadingv1alpha1.NamespaceOffloadingList
	err := c.Client.List(ctx, &namespaceOffloadings)
	runtime.Must(err)

	virtualNodes, err := liqogetters.ListVirtualNodesByLabels(ctx, c.Client, labels.Everything())
	runtime.Must(err)

	nodeNameClusterIDMap := map[string]string{}
	for i := range virtualNodes.Items {
		virtualNode := &virtualNodes.Items[i]
		clusterID := virtualNode.Spec.ClusterIdentity.ClusterID
		nodeNameClusterIDMap[virtualNode.Name] = clusterID
	}

	namespaceInfoSlice := make([]NamespaceInfo, len(namespaceOffloadings.Items))
	for i := range namespaceOffloadings.Items {
		namespaceOffloading := &namespaceOffloadings.Items[i]
		namespaceInfoSlice[i] = c.getNamespaceInfo(ctx, namespaceOffloading, nodeNameClusterIDMap)
	}
	return namespaceInfoSlice
}

func (c *Builder) getNamespaceInfo(ctx context.Context,
	namespaceOffloading *offloadingv1alpha1.NamespaceOffloading, nodeNameClusterIDMap map[string]string) NamespaceInfo {
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
	var foreignClusterList discoveryv1alpha1.ForeignClusterList
	err := c.Client.List(ctx, &foreignClusterList)
	runtime.Must(err)

	peeringInfoSlice := make([]PeeringInfo, len(foreignClusterList.Items))
	for i := range foreignClusterList.Items {
		foreignCluster := &foreignClusterList.Items[i]
		peeringInfoSlice[i] = c.getPeeringInfo(ctx, foreignCluster)
	}
	return peeringInfoSlice
}

func (c *Builder) getPeeringInfo(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) PeeringInfo {
	discoveryType := discovery.ManualDiscovery
	if v, ok := foreignCluster.Labels[discovery.DiscoveryTypeLabel]; ok {
		discoveryType = discovery.Type(v)
	}

	var latency time.Duration
	tunnel, err := liqogetters.GetTunnelEndpoint(ctx, c.Client,
		&foreignCluster.Spec.ClusterIdentity, foreignCluster.Status.TenantNamespace.Local)
	switch {
	case apierrors.IsNotFound(err):
		// do nothing
	case err != nil:
		klog.Errorf("unable to get tunnel endpoint for cluster %q: %v", foreignCluster.Spec.ClusterIdentity.ClusterName, err)
	default:
		latency, err = time.ParseDuration(tunnel.Status.Connection.Latency.Value)
		if err != nil {
			klog.Errorf("unable to parse latency for cluster %q: %v", foreignCluster.Spec.ClusterIdentity.ClusterName, err)
		}
	}

	peeringInfo := PeeringInfo{
		RemoteClusterID: foreignCluster.Spec.ClusterIdentity.ClusterID,
		Method:          foreignCluster.Spec.PeeringType,
		DiscoveryType:   discoveryType,
		Latency:         latency,
		Incoming: c.getPeeringDetails(ctx, foreignCluster,
			discoveryv1alpha1.IncomingPeeringCondition,
			labelsutils.LocalLabelSelectorForCluster(foreignCluster.Spec.ClusterIdentity.ClusterID)),
		Outgoing: c.getPeeringDetails(ctx, foreignCluster,
			discoveryv1alpha1.OutgoingPeeringCondition,
			labelsutils.RemoteLabelSelectorForCluster(foreignCluster.Spec.ClusterIdentity.ClusterID)),
	}
	return peeringInfo
}

func (c *Builder) getPeeringDetails(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster,
	condition discoveryv1alpha1.PeeringConditionType, selector labels.Selector) PeeringDetails {
	enabled := peeringconditionsutils.GetStatus(foreignCluster,
		condition) == discoveryv1alpha1.PeeringConditionStatusEstablished

	var resources corev1.ResourceList
	if enabled && foreignCluster.Status.TenantNamespace.Local != "" {
		offer, err := liqogetters.GetResourceOfferByLabel(ctx, c.Client,
			foreignCluster.Status.TenantNamespace.Local,
			selector)
		if err != nil {
			klog.Errorf("unable to get resource offer for cluster %s: %v",
				foreignCluster.Spec.ClusterIdentity.ClusterID, err)
			return PeeringDetails{
				Enabled:   enabled,
				Resources: corev1.ResourceList{},
			}
		}
		resources = offer.Spec.ResourceQuota.Hard
	}

	return PeeringDetails{
		Enabled:   enabled,
		Resources: resources,
	}
}
