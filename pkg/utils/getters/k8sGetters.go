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

// Package getters provides functions to get k8s resources and liqo custom resources.
package getters

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	liqolabels "github.com/liqotech/liqo/pkg/utils/labels"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

// GetNamespaceMapByLabel returns the NamespaceMapping with the given labels.
func GetNamespaceMapByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*offloadingv1beta1.NamespaceMap, error) {
	var namespaceMapList offloadingv1beta1.NamespaceMapList
	if err := cl.List(ctx, &namespaceMapList, client.MatchingLabelsSelector{Selector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	switch len(namespaceMapList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(offloadingv1beta1.NamespaceMapGroupResource, offloadingv1beta1.NamespaceMapResource)
	case 1:
		return &namespaceMapList.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type %q found for label selector %q,"+
			" when only one was expected", offloadingv1beta1.NamespaceMapGroupResource.String(), lSelector.String())
	}
}

// ListNamespaceMapsByLabel returns the NamespaceMaps that match the given label selector.
func ListNamespaceMapsByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) ([]offloadingv1beta1.NamespaceMap, error) {
	var namespaceMapList offloadingv1beta1.NamespaceMapList
	if err := cl.List(ctx, &namespaceMapList, client.MatchingLabelsSelector{Selector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	return namespaceMapList.Items, nil
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

// ListRolesByLabel returns the Roles that match the given label selector.
func ListRolesByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) ([]rbacv1.Role, error) {
	var rList rbacv1.RoleList
	if err := cl.List(ctx, &rList, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	return rList.Items, nil
}

// ListClusterRolesByLabel returns the ClusterRoles that match the given label selector.
func ListClusterRolesByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) ([]rbacv1.ClusterRole, error) {
	var crList rbacv1.ClusterRoleList
	if err := cl.List(ctx, &crList, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return crList.Items, nil
}

// ListRoleBindingsByLabel returns the RoleBindings that match the given label selector.
func ListRoleBindingsByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) ([]rbacv1.RoleBinding, error) {
	var rbList rbacv1.RoleBindingList
	if err := cl.List(ctx, &rbList, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	return rbList.Items, nil
}

// ListClusterRoleBindingsByLabel returns the ClusterRoleBindings that match the given label selector.
func ListClusterRoleBindingsByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) ([]rbacv1.ClusterRoleBinding, error) {
	var crbList rbacv1.ClusterRoleBindingList
	if err := cl.List(ctx, &crbList, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return crbList.Items, nil
}

// ListNodesByClusterID returns the node list that matches the given cluster id.
func ListNodesByClusterID(ctx context.Context, cl client.Client, clusterID liqov1beta1.ClusterID) (*corev1.NodeList, error) {
	list := new(corev1.NodeList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: string(clusterID),
		}),
	}); err != nil {
		return nil, err
	}

	nodeRN := "nodes"
	nodeGR := corev1.Resource(nodeRN)

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(nodeGR, string(clusterID))
	default:
		return list, nil
	}
}

// GetNonceSecretByClusterID returns the secret containing the nonce to be signed by the consumer cluster.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func GetNonceSecretByClusterID(ctx context.Context, cl client.Client, remoteClusterID liqov1beta1.ClusterID,
	tenantNamespace string) (*corev1.Secret, error) {
	var secrets corev1.SecretList

	if err := cl.List(ctx, &secrets, &client.ListOptions{
		Namespace: tenantNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID:     string(remoteClusterID),
			consts.NonceSecretLabelKey: "true",
		}),
	}); err != nil {
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		return nil, kerrors.NewNotFound(corev1.Resource(string(corev1.ResourceSecrets)), string(remoteClusterID))
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple nonce secrets found for remote cluster %q", remoteClusterID)
	}
}

// GetSignedNonceSecretByClusterID returns the secret containing the nonce signed by the consumer cluster.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func GetSignedNonceSecretByClusterID(
	ctx context.Context, cl client.Client, remoteClusterID liqov1beta1.ClusterID, tenantNamespace string) (*corev1.Secret, error) {
	var secrets corev1.SecretList

	if err := cl.List(ctx, &secrets, &client.ListOptions{
		Namespace: tenantNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID:           string(remoteClusterID),
			consts.SignedNonceSecretLabelKey: "true",
		}),
	}); err != nil {
		return nil, err
	}

	switch len(secrets.Items) {
	case 0:
		return nil, kerrors.NewNotFound(corev1.Resource(string(corev1.ResourceSecrets)), string(remoteClusterID))
	case 1:
		return &secrets.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple signed nonce secrets found for remote cluster %q", remoteClusterID)
	}
}

// GetTenantByClusterID returns the Tenant resource for the given cluster id.
func GetTenantByClusterID(
	ctx context.Context, cl client.Client, clusterID liqov1beta1.ClusterID, tenantNamespace string) (*authv1beta1.Tenant, error) {
	list := new(authv1beta1.TenantList)
	if err := cl.List(ctx, list, &client.ListOptions{
		Namespace: tenantNamespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: string(clusterID),
		}),
	}); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(authv1beta1.TenantGroupResource, string(clusterID))
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for cluster {%s},"+
			" when only one was expected", authv1beta1.TenantResource, clusterID)
	}
}

// GetTenantByName returns the Tenant resource given its name.
func GetTenantByName(
	ctx context.Context, cl client.Client, name string, tenantNamespace string) (*authv1beta1.Tenant, error) {
	list := new(authv1beta1.TenantList)
	if err := cl.List(
		ctx, list,
		client.InNamespace(tenantNamespace),
		client.MatchingFields{"metadata.name": name},
	); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(authv1beta1.TenantGroupResource, name)
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple resources of type {%s} found for cluster {%s},"+
			" when only one was expected", authv1beta1.TenantResource, name)
	}
}

// GetControlPlaneIdentityByClusterID returns the Identity of type ControlPlane for the given cluster id.
func GetControlPlaneIdentityByClusterID(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID) (*authv1beta1.Identity, error) {
	list := new(authv1beta1.IdentityList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: string(clusterID),
		}),
	}); err != nil {
		return nil, err
	}

	var controlPlaneIdentity *authv1beta1.Identity
	found := false
	for i := range list.Items {
		if list.Items[i].Spec.Type == authv1beta1.ControlPlaneIdentityType {
			if found {
				return nil, fmt.Errorf("multiple resources of type {%s} found for cluster {%s},"+
					" when only one was expected", authv1beta1.IdentityResource, clusterID)
			}
			controlPlaneIdentity = &list.Items[i]
			found = true
		}
	}
	if !found {
		return nil, kerrors.NewNotFound(authv1beta1.IdentityGroupResource, string(clusterID))
	}

	return controlPlaneIdentity, nil
}

// GetResourceSliceIdentitiesByClusterID returns the list of Identities of type ResourceSlice for the given cluster id.
func GetResourceSliceIdentitiesByClusterID(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID) ([]authv1beta1.Identity, error) {
	list := new(authv1beta1.IdentityList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID: string(clusterID),
		}),
	}); err != nil {
		return nil, err
	}

	var identities []authv1beta1.Identity
	for i := range list.Items {
		if list.Items[i].Spec.Type == authv1beta1.ResourceSliceIdentityType {
			identities = append(identities, list.Items[i])
		}
	}

	return identities, nil
}

// GetIdentityFromResourceSlice returns the Identity of type ResourceSlice for the given cluster id and resourceslice name.
func GetIdentityFromResourceSlice(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID, resourceSliceName string) (*authv1beta1.Identity, error) {
	identities, err := GetResourceSliceIdentitiesByClusterID(ctx, cl, clusterID)
	if err != nil {
		return nil, err
	}

	for i := range identities {
		if identities[i].Labels != nil && identities[i].Labels[consts.ResourceSliceNameLabelKey] == resourceSliceName {
			return &identities[i], nil
		}
	}

	return nil, kerrors.NewNotFound(authv1beta1.IdentityGroupResource, string(clusterID))
}

// GetControlPlaneKubeconfigSecretByClusterID returns the Secret containing the Kubeconfig of
// a ControlPlane Identity given the cluster id.
func GetControlPlaneKubeconfigSecretByClusterID(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID) (*corev1.Secret, error) {
	list := new(corev1.SecretList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID:      string(clusterID),
			consts.IdentityTypeLabelKey: string(authv1beta1.ControlPlaneIdentityType),
		}),
	}); err != nil {
		return nil, err
	}

	switch len(list.Items) {
	case 0:
		return nil, kerrors.NewNotFound(corev1.Resource(string(corev1.ResourceSecrets)), string(clusterID))
	case 1:
		return &list.Items[0], nil
	default:
		return nil, fmt.Errorf("found multiple secrets containing ControlPlane Kubeconfig for cluster %s", clusterID)
	}
}

// GetResourceSliceKubeconfigSecretsByClusterID returns the list of Secrets containing the Kubeconfig of
// a ResourceSlice Identity given the cluster id.
func GetResourceSliceKubeconfigSecretsByClusterID(ctx context.Context, cl client.Client, clusterID string) ([]corev1.Secret, error) {
	list := new(corev1.SecretList)
	if err := cl.List(ctx, list, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			consts.RemoteClusterID:      clusterID,
			consts.IdentityTypeLabelKey: string(authv1beta1.ResourceSliceIdentityType),
		}),
	}); err != nil {
		return nil, err
	}

	return list.Items, nil
}

// ListResourceSlicesByLabel returns the ResourceSlice list that matches the given label selector.
func ListResourceSlicesByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) ([]authv1beta1.ResourceSlice, error) {
	var list authv1beta1.ResourceSliceList
	if err := cl.List(ctx, &list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListResourceSlicesByClusterID returns the list of ResourceSlices for the given cluster id.
func ListResourceSlicesByClusterID(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID) ([]authv1beta1.ResourceSlice, error) {
	resSlices, err := ListResourceSlicesByLabel(ctx, cl, corev1.NamespaceAll, labels.SelectorFromSet(map[string]string{
		consts.RemoteClusterID: string(remoteClusterID),
	}))
	if err != nil {
		return nil, err
	}
	return resSlices, nil
}

// GetKubeconfigSecretFromIdentity returns the Secret referenced in the status of the given Identity.
func GetKubeconfigSecretFromIdentity(ctx context.Context, cl client.Client, identity *authv1beta1.Identity) (*corev1.Secret, error) {
	if identity.Status.KubeconfigSecretRef == nil || identity.Status.KubeconfigSecretRef.Name == "" {
		return nil, fmt.Errorf("identity %q does not contain the kubeconfig secret reference", identity.Name)
	}

	var kubeconfigSecret corev1.Secret
	err := cl.Get(ctx, client.ObjectKey{Name: identity.Status.KubeconfigSecretRef.Name, Namespace: identity.Namespace}, &kubeconfigSecret)
	if err != nil {
		return nil, fmt.Errorf("unable to get the kubeconfig secret %q: %w", identity.Status.KubeconfigSecretRef.Name, err)
	}

	return &kubeconfigSecret, nil
}

// ListShadowPodsByCreator returns the list of ShadowPods created by the given user.
func ListShadowPodsByCreator(ctx context.Context, cl client.Client, creator string) (*offloadingv1beta1.ShadowPodList, error) {
	list := new(offloadingv1beta1.ShadowPodList)
	if err := cl.List(ctx, list, client.MatchingLabels{consts.CreatorLabelKey: creator}); err != nil {
		return nil, err
	}
	return list, nil
}

// GetQuotaByUser returns the list of Quotas for the given user.
func GetQuotaByUser(ctx context.Context, cl client.Client,
	user string) (*offloadingv1beta1.Quota, error) {
	var quotas offloadingv1beta1.QuotaList
	err := cl.List(ctx, &quotas, &client.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range quotas.Items {
		if quotas.Items[i].Spec.User == user {
			return &quotas.Items[i], nil
		}
	}

	return nil, kerrors.NewNotFound(offloadingv1beta1.QuotaGroupResource, user)
}

// GetOffloadingByNamespace returns the NamespaceOffloading resource for the given namespace.
func GetOffloadingByNamespace(ctx context.Context, cl client.Client, namespace string) (*offloadingv1beta1.NamespaceOffloading, error) {
	var nsOffloading offloadingv1beta1.NamespaceOffloading
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
func ListVirtualNodesByLabels(ctx context.Context, cl client.Client, lSelector labels.Selector) (*offloadingv1beta1.VirtualNodeList, error) {
	var virtualNodes offloadingv1beta1.VirtualNodeList
	err := cl.List(ctx, &virtualNodes, &client.ListOptions{LabelSelector: lSelector})
	return &virtualNodes, err
}

// ListVirtualNodesByClusterID returns the list of virtual nodes for the given cluster id.
func ListVirtualNodesByClusterID(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID) ([]offloadingv1beta1.VirtualNode, error) {
	virtualNodes, err := ListVirtualNodesByLabels(ctx, cl, labels.SelectorFromSet(map[string]string{
		consts.RemoteClusterID: string(remoteClusterID),
	}))
	if err != nil {
		return nil, err
	}
	return virtualNodes.Items, nil
}

// GetNodeFromVirtualNode returns the node object from the given virtual node name.
func GetNodeFromVirtualNode(ctx context.Context, cl client.Client, virtualNode *offloadingv1beta1.VirtualNode) (*corev1.Node, error) {
	nodename := virtualNode.Name
	nodes, err := ListNodesByClusterID(ctx, cl, virtualNode.Spec.ClusterID)
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

// MapForeignClustersByLabel returns a map of foreign clusters indexed their names.
func MapForeignClustersByLabel(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (map[string]liqov1beta1.ForeignCluster, error) {
	result := make(map[string]liqov1beta1.ForeignCluster)
	list := new(liqov1beta1.ForeignClusterList)
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
	vn *offloadingv1beta1.VirtualNode) (*corev1.PodList, error) {
	list := &corev1.PodList{}
	vklabels := vkforge.VirtualKubeletLabels(vn)
	err := cl.List(ctx, list, client.MatchingLabels(vklabels))
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetControllerManagerDeployment returns the liqo controller manager deployment.
func GetControllerManagerDeployment(ctx context.Context, cl client.Client, liqoNamespace string) (*appsv1.Deployment, error) {
	var deployments appsv1.DeploymentList
	if err := cl.List(ctx, &deployments, client.InNamespace(liqoNamespace), client.MatchingLabelsSelector{
		Selector: liqolabels.ControllerManagerLabelSelector(),
	}); err != nil {
		return nil, errors.New("failed to retrieve the liqo controller manager deployment")
	}

	switch len(deployments.Items) {
	case 0:
		return nil, kerrors.NewNotFound(appsv1.Resource("deployments"), "liqo-controller-manager")
	case 1:
		return &deployments.Items[0], nil
	default:
		return nil, fmt.Errorf("retrieved multiple liqo-controller-manager deployments: %v", len(deployments.Items))
	}
}

// GetControllerManagerDeploymentWithDynamicClient retrieves the controller manager deployment usingg the dynamic client.
func GetControllerManagerDeploymentWithDynamicClient(ctx context.Context,
	dynClient dynamic.Interface, liqoNamespace string) (*appsv1.Deployment, error) {
	unstr, err := dynClient.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(liqoNamespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: liqolabels.ControllerManagerLabelSelector().String(),
		})
	if err != nil {
		return nil, err
	}

	var deployments appsv1.DeploymentList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &deployments); err != nil {
		return nil, err
	}

	switch len(deployments.Items) {
	case 0:
		return nil, kerrors.NewNotFound(appsv1.Resource("deployments"), "liqo-controller-manager")
	case 1:
		return &deployments.Items[0], nil
	default:
		return nil, fmt.Errorf("retrieved multiple liqo-controller-manager deployments: %v", len(deployments.Items))
	}
}

// GetContainerImageVersion gets the version of the image of the container with the given name from a list of containers.
func GetContainerImageVersion(containers []corev1.Container, containerName string) (version string, err error) {
	for i := range containers {
		if containers[i].Name == containerName {
			imageSplit := strings.Split(containers[i].Image, ":")
			if len(imageSplit) == 1 {
				return "", errors.New("missing version in container image")
			}
			return imageSplit[len(imageSplit)-1], nil
		}
	}
	return "", fmt.Errorf("no container with name %q found", containerName)
}

// GetLiqoVersion returns the installed Liqo version.
func GetLiqoVersion(ctx context.Context, cl client.Client, liqoNamespace string) (string, error) {
	deployment, err := GetControllerManagerDeployment(ctx, cl, liqoNamespace)
	if err != nil {
		return "", err
	}

	// Get version from image version
	containers := deployment.Spec.Template.Spec.Containers
	return GetContainerImageVersion(containers, "controller-manager")
}

// ListNetworksByLabel returns the Network resource with the given labels.
func ListNetworksByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*ipamv1alpha1.NetworkList, error) {
	list := &ipamv1alpha1.NetworkList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListPublicKeysByLabel returns the PublicKey resource with the given labels.
func ListPublicKeysByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*networkingv1beta1.PublicKeyList, error) {
	list := &networkingv1beta1.PublicKeyList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListConnectionsByLabel returns the Connection resource with the given labels.
func ListConnectionsByLabel(ctx context.Context, cl client.Client, ns string, lSelector labels.Selector) (*networkingv1beta1.ConnectionList, error) {
	list := &networkingv1beta1.ConnectionList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListRouteConfigurationsByLabel returns the RouteConfiguration resource with the given labels.
func ListRouteConfigurationsByLabel(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1beta1.RouteConfigurationList, error) {
	list := &networkingv1beta1.RouteConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector})
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListRouteConfigurationsInNamespaceByLabel returns the RouteConfiguration resource in a namespace with the given labels.
func ListRouteConfigurationsInNamespaceByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*networkingv1beta1.RouteConfigurationList, error) {
	list := &networkingv1beta1.RouteConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListFirewallConfigurationsByLabel returns the FirewallConfiguration resource with the given labels.
func ListFirewallConfigurationsByLabel(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1beta1.FirewallConfigurationList, error) {
	list := &networkingv1beta1.FirewallConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector})
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListFirewallConfigurationsInNamespaceByLabel returns the FirewallConfiguration resource with the given labels.
func ListFirewallConfigurationsInNamespaceByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*networkingv1beta1.FirewallConfigurationList, error) {
	list := &networkingv1beta1.FirewallConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListConfigurationsInNamespaceByLabel returns the Configuration resources with the given labels.
func ListConfigurationsInNamespaceByLabel(ctx context.Context, cl client.Client,
	ns string, lSelector labels.Selector) (*networkingv1beta1.ConfigurationList, error) {
	list := &networkingv1beta1.ConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}, client.InNamespace(ns))
	if err != nil {
		return nil, err
	}
	return list, err
}

// ListConfigurationsByLabel returns the Configuration resource with the given labels.
func ListConfigurationsByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) (*networkingv1beta1.ConfigurationList, error) {
	list := &networkingv1beta1.ConfigurationList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector})
	if err != nil {
		return nil, err
	}
	return list, err
}

// GetConfigurationByClusterID returns the Configuration resource with the given clusterID.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func GetConfigurationByClusterID(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID, namespace string) (*networkingv1beta1.Configuration, error) {
	remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: string(clusterID)}.AsSelector()

	configurations, err := ListConfigurationsInNamespaceByLabel(ctx, cl, namespace, remoteClusterIDSelector)
	if err != nil {
		return nil, err
	}

	switch len(configurations.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1beta1.ConfigurationGroupResource, networkingv1beta1.ConfigurationResource)
	case 1:
		return &configurations.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Configurations found for ForeignCluster %s", clusterID)
	}
}

// ListIPsByLabel returns the IP resource with the given labels.
func ListIPsByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector) (*ipamv1alpha1.IPList, error) {
	list := &ipamv1alpha1.IPList{}
	err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector})
	if err != nil {
		return nil, err
	}
	return list, err
}

// GetConnectionByClusterID returns the Connection resource with the given clusterID.
func GetConnectionByClusterID(ctx context.Context, cl client.Client, clusterID string) (*networkingv1beta1.Connection, error) {
	remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: clusterID}.AsSelector()
	connections, err := ListConnectionsByLabel(ctx, cl, corev1.NamespaceAll, remoteClusterIDSelector)
	if err != nil {
		return nil, err
	}

	switch len(connections.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1beta1.ConnectionGroupResource, networkingv1beta1.ConnectionResource)
	case 1:
		return &connections.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Connections found for ForeignCluster %s", clusterID)
	}
}

// GetConnectionByClusterIDInNamespace returns the Connection resource with the given clusterID in the given namespace.
func GetConnectionByClusterIDInNamespace(ctx context.Context, cl client.Client, clusterID, namespace string) (*networkingv1beta1.Connection, error) {
	remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: clusterID}.AsSelector()
	connections, err := ListConnectionsByLabel(ctx, cl, namespace, remoteClusterIDSelector)
	if err != nil {
		return nil, err
	}

	switch len(connections.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1beta1.ConnectionGroupResource, networkingv1beta1.ConnectionResource)
	case 1:
		return &connections.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple Connections found for ForeignCluster %s", clusterID)
	}
}

// GetGatewaysByClusterID returns both the GatewayServer and GatewayClient resource with the given clusterID, if not
// found it returns a nil pointer. Normally, with a common peering, this function should return only one gateway either
// the client or the server.
func GetGatewaysByClusterID(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID) (*networkingv1beta1.GatewayServer, *networkingv1beta1.GatewayClient, error) {
	gwclient, err := GetGatewayClientByClusterID(ctx, cl, remoteClusterID, corev1.NamespaceAll)
	if err != nil && !kerrors.IsNotFound(err) {
		return nil, nil, err
	}

	gwserver, err := GetGatewayServerByClusterID(ctx, cl, remoteClusterID, corev1.NamespaceAll)
	if err != nil && !kerrors.IsNotFound(err) {
		return nil, nil, err
	}

	return gwserver, gwclient, nil
}

// GetGatewayServerByClusterID returns the GatewayServer resource with the given clusterID.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func GetGatewayServerByClusterID(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID, namespace string) (*networkingv1beta1.GatewayServer, error) {
	var gwServers networkingv1beta1.GatewayServerList
	if err := cl.List(
		ctx, &gwServers,
		client.MatchingLabels{
			consts.RemoteClusterID: string(remoteClusterID),
		},
		client.InNamespace(namespace),
	); err != nil {
		return nil, err
	}

	switch len(gwServers.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1beta1.GatewayServerGroupResource, networkingv1beta1.GatewayServerResource)
	case 1:
		return &gwServers.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple GatewayServers found for ForeignCluster %s", remoteClusterID)
	}
}

// GetGatewayClientByClusterID returns the GatewayClient resource with the given clusterID.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func GetGatewayClientByClusterID(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID, namespace string) (*networkingv1beta1.GatewayClient, error) {
	var gwClients networkingv1beta1.GatewayClientList
	if err := cl.List(
		ctx, &gwClients,
		client.MatchingLabels{
			consts.RemoteClusterID: string(remoteClusterID),
		},
		client.InNamespace(namespace),
	); err != nil {
		return nil, err
	}

	switch len(gwClients.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1beta1.GatewayClientGroupResource, networkingv1beta1.GatewayClientResource)
	case 1:
		return &gwClients.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple GatewayClients found for ForeignCluster %s", remoteClusterID)
	}
}

// GetPublicKeyByClusterID returns the PublicKey resource with the given clusterID.
func GetPublicKeyByClusterID(ctx context.Context, cl client.Client,
	remoteClusterID liqov1beta1.ClusterID, namespace string) (*networkingv1beta1.PublicKey, error) {
	publicKeys, err := ListPublicKeysByLabel(ctx, cl, namespace, labels.SelectorFromSet(map[string]string{
		consts.RemoteClusterID: string(remoteClusterID),
	}))
	if err != nil {
		return nil, err
	}

	switch len(publicKeys.Items) {
	case 0:
		return nil, kerrors.NewNotFound(networkingv1beta1.PublicKeyGroupResource, networkingv1beta1.PublicKeyResource)
	case 1:
		return &publicKeys.Items[0], nil
	default:
		return nil, fmt.Errorf("multiple PublicKeys found for ForeignCluster %s", remoteClusterID)
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

// ListLiqoNodes returns the list of nodes of type virtual-node.
func ListLiqoNodes(ctx context.Context, cl client.Client) (*corev1.NodeList, error) {
	req, err := labels.NewRequirement(consts.TypeLabel, selection.Equals, []string{consts.TypeNode})
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

// ListNotLiqoNodes returns the list of nodes not created by Liqo.
func ListNotLiqoNodes(ctx context.Context, cl client.Client) (*corev1.NodeList, error) {
	req, err := labels.NewRequirement(consts.TypeLabel, selection.DoesNotExist, []string{})
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
	lSelector labels.Selector) (*networkingv1beta1.InternalNodeList, error) {
	list := new(networkingv1beta1.InternalNodeList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// ListInternalFabricsByLabels returns the list of internalfabrics resources.
func ListInternalFabricsByLabels(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1beta1.InternalFabricList, error) {
	list := new(networkingv1beta1.InternalFabricList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// ListGeneveTunnelsByLabels returns the list of genevetunnels resources.
func ListGeneveTunnelsByLabels(ctx context.Context, cl client.Client,
	lSelector labels.Selector) (*networkingv1beta1.GeneveTunnelList, error) {
	list := new(networkingv1beta1.GeneveTunnelList)
	if err := cl.List(ctx, list, &client.ListOptions{LabelSelector: lSelector}); err != nil {
		return nil, err
	}
	return list, nil
}

// GetUniqueNetworkByLabel retrieves the Network resource with the given label selector.
// It returns error if multiple resources are found.
func GetUniqueNetworkByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector, namespace string) (*ipamv1alpha1.Network, error) {
	networks, err := GetNetworksByLabel(ctx, cl, lSelector, namespace)
	if err != nil {
		return nil, err
	}

	switch len(networks) {
	case 0:
		return nil, kerrors.NewNotFound(ipamv1alpha1.NetworkGroupResource, ipamv1alpha1.NetworkResource)
	case 1:
		return &networks[0], nil
	default:
		return nil, fmt.Errorf("multiple Network resources found for label selector %q", lSelector)
	}
}

// GetNetworksByLabel retrieves the Network resources with the given labelSelector.
func GetNetworksByLabel(ctx context.Context, cl client.Client, lSelector labels.Selector, namespace string) ([]ipamv1alpha1.Network, error) {
	var networks ipamv1alpha1.NetworkList
	if err := cl.List(
		ctx,
		&networks,
		&client.ListOptions{LabelSelector: lSelector},
		client.InNamespace(namespace),
	); err != nil {
		return nil, err
	}
	return networks.Items, nil
}
