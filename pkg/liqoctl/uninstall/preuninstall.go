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

package uninstall

import (
	"context"
	"fmt"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/errors"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

type errorMap struct {
	outgoingPeering []string
	incomingPeering []string
	networking      []string
	offloading      []string
	generic         []string
}

func newErrorMap() errorMap {
	return errorMap{
		outgoingPeering: []string{},
		incomingPeering: []string{},
		networking:      []string{},
		offloading:      []string{},
		generic:         []string{},
	}
}

func (em *errorMap) getError() error {
	str := ""
	hasErr := false
	if len(em.outgoingPeering) > 0 {
		str += "\ndisable outgoing peering for clusters:\n"
		for _, fc := range em.outgoingPeering {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.incomingPeering) > 0 {
		str += "\ndisable incoming peering for clusters:\n"
		for _, fc := range em.incomingPeering {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.networking) > 0 {
		str += "\ndisable networking for clusters:\n"
		for _, fc := range em.networking {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.offloading) > 0 {
		str += "\ndisable offloading for namespaces:\n"
		for _, fc := range em.offloading {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}
	if len(em.generic) > 0 {
		str += "\nremove the following resources:\n"
		for i := range em.generic {
			str += fmt.Sprintf("- %s\n", em.generic[i])
		}
		hasErr = true
	}

	if hasErr {
		return fmt.Errorf("you should:\n%s", str)
	}
	return nil
}

func (o *Options) preUninstall(ctx context.Context) error {
	var foreignClusterList discoveryv1alpha1.ForeignClusterList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &foreignClusterList)); err != nil {
		return err
	}

	errMap := newErrorMap()

	// Search for ForeignCluster resources
	for i := range foreignClusterList.Items {
		fc := &foreignClusterList.Items[i]

		if foreignclusterutils.IsNetworkingEstablished(fc) {
			errMap.networking = append(errMap.networking, fc.Name)
		}
	}

	// Search for NamespaceOffloading resources
	var namespaceOffloadings offloadingv1alpha1.NamespaceOffloadingList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &namespaceOffloadings)); err != nil {
		return err
	}
	for i := range namespaceOffloadings.Items {
		offloading := &namespaceOffloadings.Items[i]
		errMap.offloading = append(errMap.offloading, offloading.Namespace)
	}

	// Search for Configuration resources
	var configurations networkingv1alpha1.ConfigurationList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &configurations)); err != nil {
		return err
	}
	for i := range configurations.Items {
		addResourceToErrMap(&configurations.Items[i], &errMap, &foreignClusterList)
	}

	// Search for GatewayServer resources
	var gatewayServers networkingv1alpha1.GatewayServerList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &gatewayServers)); err != nil {
		return err
	}
	for i := range gatewayServers.Items {
		addResourceToErrMap(&gatewayServers.Items[i], &errMap, &foreignClusterList)
	}

	// Search for GatewayClient resources
	var gatewayClients networkingv1alpha1.GatewayClientList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &gatewayClients)); err != nil {
		return err
	}
	for i := range gatewayClients.Items {
		addResourceToErrMap(&gatewayClients.Items[i], &errMap, &foreignClusterList)
	}

	// Search for IP resources
	var ips ipamv1alpha1.IPList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &ips)); err != nil {
		return err
	}
	for i := range ips.Items {
		if len(ips.Items[i].GetFinalizers()) > 0 {
			addResourceToErrMap(&ips.Items[i], &errMap, &foreignClusterList)
		}
	}

	// Search for Network resources
	var networks ipamv1alpha1.NetworkList
	if err := errors.IgnoreNoMatchError(o.CRClient.List(ctx, &networks)); err != nil {
		return err
	}
	for i := range networks.Items {
		// These networks will be handled by the uninstaller job
		if ipamutils.IsExternalCIDR(&networks.Items[i]) || ipamutils.IsInternalCIDR(&networks.Items[i]) {
			continue
		}
		if len(networks.Items[i].GetFinalizers()) > 0 {
			addResourceToErrMap(&networks.Items[i], &errMap, &foreignClusterList)
		}
	}

	return errMap.getError()
}

func addResourceToErrMap(obj client.Object, errMap *errorMap, foreignClusters *discoveryv1alpha1.ForeignClusterList) {
	// Check if object is a networking resource associated with a remote cluster
	clusterName, found := getRemoteClusterName(obj, foreignClusters)
	if found {
		if !slices.Contains(errMap.networking, clusterName) {
			errMap.networking = append(errMap.networking, clusterName)
		}
		return
	}

	// Add generic resource to error map
	name := fmt.Sprintf("%s: %s/%s", obj.GetObjectKind().GroupVersionKind().GroupKind(), obj.GetNamespace(), obj.GetName())
	if !slices.Contains(errMap.generic, name) {
		errMap.generic = append(errMap.generic, name)
	}
}

func getRemoteClusterName(obj client.Object, foreignClusters *discoveryv1alpha1.ForeignClusterList) (string, bool) {
	v, ok := obj.GetLabels()[consts.RemoteClusterID]
	if ok && v != "" && foreignClusters != nil {
		for i := range foreignClusters.Items {
			if foreignClusters.Items[i].Spec.ClusterIdentity.ClusterID == v {
				return foreignClusters.Items[i].Spec.ClusterIdentity.ClusterName, true
			}
		}
	}
	return "", false
}
