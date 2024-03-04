// Copyright 2019-2024 The Liqo Authors
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

// Package getters provides functions to get k8s resources and liqo custom resources.
package getters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// GetIPAMStorageByLabel it returns a IPAMStorage instance that matches the given label selector.
func GetIPAMStorageByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) (*netv1alpha1.IpamStorage, error) {
	list := new(netv1alpha1.IpamStorageList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(netv1alpha1.IpamGroupResource, netv1alpha1.ResourceIpamStorages)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for label selector {%s}"+
			" when only one was expected", netv1alpha1.IpamGroupResource.String(), lSelector.String())
	}
}

// GetNetworkConfigByLabel it returns a networkconfigs instance that matches the given label selector.
func GetNetworkConfigByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*netv1alpha1.NetworkConfig, error) {
	list, err := ListNetworkConfigsByLabel(ctx, cl, ns, lSelector)
	if err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(netv1alpha1.NetworkConfigGroupResource, netv1alpha1.ResourceNetworkConfigs)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for label selector {%s} in namespace {%s},"+
			" when only one was expected", netv1alpha1.NetworkConfigGroupResource.String(), lSelector.String(), ns)
	}
}

// ListNetworkConfigsByLabel it returns a NetworkConfig list that matches the given label selector.
func ListNetworkConfigsByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*netv1alpha1.NetworkConfigList, error) {
	list := new(netv1alpha1.NetworkConfigList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	return list, nil
}

// GetResourceOfferByLabel returns the ResourceOffer with the given labels.
func GetResourceOfferByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*sharingv1alpha1.ResourceOffer, error) {
	var resourceOfferList sharingv1alpha1.ResourceOfferList
	if err := cl.List(ctx, &resourceOfferList, client.MatchingLabelsSelector{Selector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	switch len(resourceOfferList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(sharingv1alpha1.ResourceOfferGroupResource, sharingv1alpha1.ResourceResourceOffer)
	case 1:
		return &resourceOfferList.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type %q found for label selector %q,"+
			" when only one was expected", sharingv1alpha1.ResourceOfferGroupResource.String(), lSelector.String())
	}
}

// ListResourceOfferByLabel returns the ResourceOfferList with the given labels.
func ListResourceOfferByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*sharingv1alpha1.ResourceOfferList, error) {
	list := new(sharingv1alpha1.ResourceOfferList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	return list, nil
}

// GetNamespaceMapByLabel returns the NamespaceMapping with the given labels.
func GetNamespaceMapByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*virtualkubeletv1alpha1.NamespaceMap, error) {
	var namespaceMapList virtualkubeletv1alpha1.NamespaceMapList
	if err := cl.List(ctx, &namespaceMapList, client.MatchingLabelsSelector{Selector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	switch len(namespaceMapList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(virtualkubeletv1alpha1.NamespaceMapGroupResource, virtualkubeletv1alpha1.NamespaceMapResource)
	case 1:
		return &namespaceMapList.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type %q found for label selector %q,"+
			" when only one was expected", virtualkubeletv1alpha1.NamespaceMapGroupResource.String(), lSelector.String())
	}
}

// GetServiceByLabel it returns a service instance that matches the given label selector.
func GetServiceByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*corev1.Service, error) {
	list := new(corev1.ServiceList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	svcRN := string(corev1.ResourceServices)
	svcGR := corev1.Resource(svcRN)

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(svcGR, svcRN)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for label selector {%s} in namespace {%s},"+
			" when only one was expected", svcGR.String(), lSelector.String(), ns)
	}
}

// GetSecretByLabel it returns a secret instance that matches the given label selector.
func GetSecretByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*corev1.Secret, error) {
	list := new(corev1.SecretList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	scrRN := string(corev1.ResourceSecrets)
	scrGR := corev1.Resource(scrRN)

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(scrGR, scrRN)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for label selector {%s} in namespace {%s},"+
			" when only one was expected", scrGR.String(), lSelector.String(), ns)
	}
}

// GetConfigMapByLabel it returns a configmap instance that matches the given label selector.
func GetConfigMapByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*corev1.ConfigMap, error) {
	list := new(corev1.ConfigMapList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	cmRN := string(corev1.ResourceConfigMaps)
	cmGR := corev1.Resource(cmRN)

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(cmGR, cmRN)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for label selector {%s} in namespace {%s},"+
			" when only one was expected", cmGR.String(), lSelector.String(), ns)
	}
}

// GetPodByLabel it returns a pod instance that matches the given label and field selector.
func GetPodByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector, fSelector fields.Selector) (*corev1.Pod, error) {
	list := new(corev1.PodList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: lSelector,
		FieldSelector: fSelector,
	}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	podRN := string(corev1.ResourcePods)
	podGR := corev1.Resource(podRN)

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(podGR, podRN)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for label selector {%s} in namespace {%s},"+
			" when only one was expected", podGR.String(), lSelector.String(), ns)
	}
}

// ListNodesByClusterID returns the node list that matches the given cluster id.
func ListNodesByClusterID(ctx context.Context, cl client.Client, clusterID *discoveryv1alpha1.ClusterIdentity) (*corev1.NodeList, error) {
	list := new(corev1.NodeList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: clusterID.ClusterID,
		}),
	}); err != nil {
		return nil, err
	}

	nodeRN := "nodes"
	nodeGR := corev1.Resource(nodeRN)

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(nodeGR, clusterID.ClusterID)
	default:
		return list, nil
	}
}

// GetOffloadingByNamespace returns the NamespaceOffloading resource for the given namespace.
func GetOffloadingByNamespace(ctx context.Context, cl client.Client, namespace string) (*offloadingv1alpha1.NamespaceOffloading, error) {
	var nsOffloading offloadingv1alpha1.NamespaceOffloading
	if err := cl.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      consts.DefaultNamespaceOffloadingName,
	}, &nsOffloading); err != nil {
		return nil, err
	}
	return &nsOffloading, nil
}

// ListOffloadedPods returns the list of pods offloaded from the given namespace.
func ListOffloadedPods(ctx context.Context, cl client.Client, namespace string) (corev1.PodList, error) {
	var offloadedPods corev1.PodList
	err := cl.List(ctx, &offloadedPods, client.InNamespace(namespace), client.MatchingLabels{
		consts.LocalPodLabelKey: consts.LocalPodLabelValue,
	})
	return offloadedPods, err
}

// ListVirtualNodesByLabels returns the list of virtual nodes.
func ListVirtualNodesByLabels(ctx context.Context, cl client.Client, lSelector labels.Selector) (*virtualkubeletv1alpha1.VirtualNodeList, error) {
	var virtualNodes virtualkubeletv1alpha1.VirtualNodeList
	err := cl.List(ctx, &virtualNodes, &client.ListOptions{LabelSelector: lSelector})
	return &virtualNodes, err
}

// GetNodeFromVirtualNode returns the node object from the given virtual node name.
func GetNodeFromVirtualNode(ctx context.Context, cl client.Client, virtualNode *virtualkubeletv1alpha1.VirtualNode) (*corev1.Node, error) {
	nodename := virtualNode.Name
	nodes, err := ListNodesByClusterID(ctx, cl, virtualNode.Spec.ClusterIdentity)
	if err != nil {
		return nil, err
	}
	for i := range nodes.Items {
		if nodes.Items[i].Name == nodename {
			return &nodes.Items[i], nil
		}
	}
	nodeRN := "nodes"
	nodeGR := corev1.Resource(nodeRN)
	return nil, kerrors.NewNotFound(nodeGR, nodename)
}

// GetTunnelEndpoint retrieves the tunnelEndpoint resource related to a cluster.
func GetTunnelEndpoint(ctx context.Context, cl client.Client,
	destinationClusterIdentity *discoveryv1alpha1.ClusterIdentity, namespace string) (*netv1alpha1.TunnelEndpoint, error) {
	tunEndpointList := &netv1alpha1.TunnelEndpointList{}
	lbls := client.MatchingLabels{consts.ClusterIDLabelName: destinationClusterIdentity.ClusterID}
	err := cl.List(ctx, tunEndpointList, lbls, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	switch len(tunEndpointList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(netv1alpha1.TunnelEndpointGroupResource,
			fmt.Sprintf("tunnelEndpoint for cluster: %q (ID: %s)",
				destinationClusterIdentity.ClusterName, destinationClusterIdentity.ClusterID))
	case 1:
		return &tunEndpointList.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type tunnelendpoint found for cluster %q (ID: %s)"+
			" when only one was expected", destinationClusterIdentity.ClusterName, destinationClusterIdentity.ClusterID)
	}
}

// MapForeignClustersByLabel returns a map of foreign clusters indexed their names.
func MapForeignClustersByLabel(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (map[string]discoveryv1alpha1.ForeignCluster, error) {
	result := make(map[string]discoveryv1alpha1.ForeignCluster)
	list := new(discoveryv1alpha1.ForeignClusterList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	for i := range list.Items {
		result[list.Items[i].Name] = list.Items[i]
	}
	return result, nil
}

// ListVirtualKubeletPodsFromVirtualNode returns the list of pods running a VirtualNode's VirtualKubelet.
func ListVirtualKubeletPodsFromVirtualNode(ctx context.Context, cl client.Client,
	vn *virtualkubeletv1alpha1.VirtualNode, vkopt *vkforge.VirtualKubeletOpts) (*corev1.PodList, error) {
	list := &corev1.PodList{}
	vklabels := vkforge.VirtualKubeletLabels(vn, vkopt)
	err := cl.List(ctx, list, client.MatchingLabels(vklabels))
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetLiqoVersion returns the installed Liqo version.
func GetLiqoVersion(ctx context.Context, cl client.Client, liqoNamespace string) (string, error) {
	// Retrieve the deployment of the liqo controller manager component
	var deployments appsv1.DeploymentList
	if err := cl.List(ctx, &deployments, client.InNamespace(liqoNamespace), client.MatchingLabelsSelector{
		Selector: liqolabels.ControllerManagerLabelSelector(),
	}); err != nil || len(deployments.Items) != 1 {
		return "", errors.New("failed to retrieve the liqo controller manager deployment")
	}

	// Get version from image version
	containers := deployments.Items[0].Spec.Template.Spec.Containers
	for i := range containers {
		if containers[i].Name == "controller-manager" {
			version := strings.Split(containers[i].Image, ":")[1]
			if version == "" {
				return "", errors.New("missing version in liqo controller manager image")
			}
			return version, nil
		}
	}

	return "", errors.New("retrieved an invalid liqo controller manager deployment")
}

// ListNetworkByLabel returns the Network resource with the given labels.
func ListNetworkByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*ipamv1alpha1.NetworkList, error) {
	list := &ipamv1alpha1.NetworkList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListPublicKeysByLabel returns the PublicKey resource with the given labels.
func ListPublicKeysByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*networkingv1alpha1.PublicKeyList, error) {
	list := &networkingv1alpha1.PublicKeyList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListConnectionsByLabel returns the Connection resource with the given labels.
func ListConnectionsByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*networkingv1alpha1.ConnectionList, error) {
	list := &networkingv1alpha1.ConnectionList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListRouteConfigurationsByLabel returns the RouteConfiguration resource with the given labels.
func ListRouteConfigurationsByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*networkingv1alpha1.RouteConfigurationList, error) {
	list := &networkingv1alpha1.RouteConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListFirewallConfigurationsByLabel returns the FirewallConfiguration resource with the given labels.
func ListFirewallConfigurationsByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*networkingv1alpha1.FirewallConfigurationList, error) {
	list := &networkingv1alpha1.FirewallConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListConfigurationsByLabel returns the Configuration resource with the given labels.
func ListConfigurationsByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*networkingv1alpha1.ConfigurationList, error) {
	list := &networkingv1alpha1.ConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// GetConfigurationByClusterID returns the Configuration resource with the given clusterID.
func GetConfigurationByClusterID(ctx context.Context, cl client.Client, clusterID string) (*networkingv1alpha1.Configuration, error) {
	remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: clusterID}.AsSelector()
	configurations, err := ListConfigurationsByLabel(ctx, cl, corev1.NamespaceAll, remoteClusterIDSelector)
	if err != nil {
		return nil, err
	}

	switch len(configurations.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1alpha1.ConfigurationGroupResource, networkingv1alpha1.ConfigurationResource)
	case 1:
		return &configurations.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Configurations found for ForeignCluster %s", clusterID)
	}
}

// GetConnectionByClusterID returns the Connection resource with the given clusterID.
func GetConnectionByClusterID(ctx context.Context, cl client.Client, clusterID string) (*networkingv1alpha1.Connection, error) {
	remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: clusterID}.AsSelector()
	connections, err := ListConnectionsByLabel(ctx, cl, corev1.NamespaceAll, remoteClusterIDSelector)
	if err != nil {
		return nil, err
	}

	switch len(connections.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1alpha1.ConnectionGroupResource, networkingv1alpha1.ConnectionResource)
	case 1:
		return &connections.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Connections found for ForeignCluster %s", clusterID)
	}
}

// GetGatewayServerByClusterID returns the GatewayServer resource with the given clusterID.
func GetGatewayServerByClusterID(ctx context.Context, cl client.Client,
	remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity) (*networkingv1alpha1.GatewayServer, error) {
	var gwServers networkingv1alpha1.GatewayServerList
	if err := cl.List(ctx, &gwServers, client.MatchingLabels{
		consts.RemoteClusterID: remoteClusterIdentity.ClusterID,
	}); err != nil {
		return nil, err
	}

	switch len(gwServers.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1alpha1.GatewayServerGroupResource, networkingv1alpha1.GatewayServerResource)
	case 1:
		return &gwServers.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple GatewayServers found for ForeignCluster %s", remoteClusterIdentity.ClusterID)
	}
}

// GetGatewayClientByClusterID returns the GatewayClient resource with the given clusterID.
func GetGatewayClientByClusterID(ctx context.Context, cl client.Client,
	remoteClusterIdentity *discoveryv1alpha1.ClusterIdentity) (*networkingv1alpha1.GatewayClient, error) {
	var gwClients networkingv1alpha1.GatewayClientList
	if err := cl.List(ctx, &gwClients, client.MatchingLabels{
		consts.RemoteClusterID: remoteClusterIdentity.ClusterID,
	}); err != nil {
		return nil, err
	}

	switch len(gwClients.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1alpha1.GatewayClientGroupResource, networkingv1alpha1.GatewayClientResource)
	case 1:
		return &gwClients.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple GatewayClients found for ForeignCluster %s", remoteClusterIdentity.ClusterID)
	}
}

// ListPhysicalNodes returns the list of physical nodes. (i.e. nodes not created by Liqo).
func ListPhysicalNodes(ctx context.Context, cl client.Client) (*corev1.NodeList, error) {
	req, err := labels.NewRequirement(consts.TypeLabel, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}

	lSelector := labels.NewSelector().Add(*req)

	list := new(corev1.NodeList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// ListInternalNodesByLabels returns the list of internalnodes resources. (i.e. nodes created by Liqo).
func ListInternalNodesByLabels(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1alpha1.InternalNodeList, error) {
	list := new(networkingv1alpha1.InternalNodeList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// ListInternalFabricsByLabels returns the list of internalfabrics resources.
func ListInternalFabricsByLabels(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1alpha1.InternalFabricList, error) {
	list := new(networkingv1alpha1.InternalFabricList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// ListGeneveTunnelsByLabels returns the list of genevetunnels resources.
func ListGeneveTunnelsByLabels(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1alpha1.GeneveTunnelList, error) {
	list := new(networkingv1alpha1.GeneveTunnelList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// GetUniqueNetworkByLabel retrieves the Network resource with the given label selector.
// It returns error if multiple resources are found.
func GetUniqueNetworkByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) (*ipamv1alpha1.Network, error) {
	networks, err := GetNetworksByLabel(ctx, cl, lSelector)
	if err != nil {
		return nil, err
	}

	switch len(networks.Items) {
	case 0:
		return nil, kerrors.NewNotFound(ipamv1alpha1.NetworkGroupResource, ipamv1alpha1.NetworkResource)
	case 1:
		return &networks.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Network resources found for label selector %q", lSelector)
	}
}

// GetNetworksByLabel retrieves the Network resources with the given labelSelector.
func GetNetworksByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) (*ipamv1alpha1.NetworkList, error) {
	var networks ipamv1alpha1.NetworkList
	if err := cl.List(ctx, &networks, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return &networks, nil
}
