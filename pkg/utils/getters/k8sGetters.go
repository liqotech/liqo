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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
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

// GetNonceByClusterID returns the nonce secret for the given cluster id.
func GetNonceByClusterID(ctx context.Context, cl client.Client, clusterID string) (*corev1.Secret, error) {
	list := new(corev1.SecretList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID:     clusterID,
			consts.NonceSecretLabelKey: "true",
		}),
	}); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(corev1.Resource(string(corev1.ResourceSecrets)), clusterID)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for cluster {%s},"+
			" when only one was expected", corev1.ResourceSecrets, clusterID)
	}
}

// GetTenantByClusterID returns the Tenant resource for the given cluster id.
func GetTenantByClusterID(ctx context.Context, cl client.Client, clusterID string) (*authv1alpha1.Tenant, error) {
	list := new(authv1alpha1.TenantList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: clusterID,
		}),
	}); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(authv1alpha1.TenantGroupResource, clusterID)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for cluster {%s},"+
			" when only one was expected", authv1alpha1.TenantResource, clusterID)
	}
}

// GetControlPlaneIdentityByClusterID returns the Identity of type ControlPlane for the given cluster id.
func GetControlPlaneIdentityByClusterID(ctx context.Context, cl client.Client, clusterID string) (*authv1alpha1.Identity, error) {
	list := new(authv1alpha1.IdentityList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: clusterID,
		}),
	}); err != nil {
		return nil, err
	}

	var controlPlaneIdentity *authv1alpha1.Identity
	found := false
	for i := range list.Items {
		if list.Items[i].Spec.Type == authv1alpha1.ControlPlaneIdentityType {
			if found {
				return nil, fmt.Errorf("multiple resources of type {%s} found for cluster {%s},"+
					" when only one was expected", authv1alpha1.IdentityResource, clusterID)
			}
			controlPlaneIdentity = &list.Items[i]
			found = true
		}
	}
	if !found {
		return nil, kerrors.NewNotFound(authv1alpha1.IdentityGroupResource, clusterID)
	}

	return controlPlaneIdentity, nil
}

// GetControlPlaneKubeconfigSecretByClusterID returns the Secret containing the Kubeconfig of a ControlPlane Identity given the cluster id.
func GetControlPlaneKubeconfigSecretByClusterID(ctx context.Context, cl client.Client, clusterID string) (*corev1.Secret, error) {
	list := new(corev1.SecretList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID:      clusterID,
			consts.IdentityTypeLabelKey: string(authv1alpha1.ControlPlaneIdentityType),
		}),
	}); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(corev1.Resource(string(corev1.ResourceSecrets)), clusterID)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("found multiple secrets containing ControlPlane Kubeconfig for cluster %s", clusterID)
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
