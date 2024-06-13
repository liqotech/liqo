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

package utils

import (
	"context"
	"fmt"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/errors"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
	ipsutils "github.com/liqotech/liqo/pkg/utils/ipam/ips"
)

type errorMap struct {
	networking     []string
	authentication []string
	offloading     []string
	namespaces     []string
	generic        []string
}

func newErrorMap() errorMap {
	return errorMap{
		networking:     []string{},
		authentication: []string{},
		offloading:     []string{},
		namespaces:     []string{},
		generic:        []string{},
	}
}

func (em *errorMap) getError() error {
	str := ""
	hasErr := false

	if len(em.networking) > 0 {
		str += "\ndisable networking for clusters:\n"
		for _, fc := range em.networking {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}

	if len(em.authentication) > 0 {
		str += "\ndisable authentication for clusters:\n"
		for i := range em.authentication {
			str += fmt.Sprintf("- %s\n", em.authentication[i])
		}
		hasErr = true
	}

	if len(em.offloading) > 0 {
		str += "\ndisable offloading for clusters:\n"
		for _, fc := range em.offloading {
			str += fmt.Sprintf("- %s\n", fc)
		}
		hasErr = true
	}

	if len(em.namespaces) > 0 {
		str += "\nunoffload the following namespaces:\n"
		for i := range em.namespaces {
			str += fmt.Sprintf("- %s\n", em.namespaces[i])
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

// PreUninstall checks if there are resources that need to be removed before uninstalling Liqo.
func PreUninstall(ctx context.Context, cl client.Client) error {
	var foreignClusterList discoveryv1alpha1.ForeignClusterList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &foreignClusterList)); err != nil {
		return err
	}

	errMap := newErrorMap()

	// Search for ForeignCluster resources
	for i := range foreignClusterList.Items {
		fc := &foreignClusterList.Items[i]

		if fcutils.IsNetworkingModuleEnabled(fc) {
			errMap.networking = append(errMap.networking, string(fc.Spec.ClusterID))
		}

		if fcutils.IsAuthenticationModuleEnabled(fc) {
			errMap.authentication = append(errMap.authentication, string(fc.Spec.ClusterID))
		}

		if fcutils.IsOffloadingModuleEnabled(fc) {
			errMap.offloading = append(errMap.offloading, string(fc.Spec.ClusterID))
		}
	}

	// Search for NamespaceOffloading resources
	var namespaceOffloadings offloadingv1alpha1.NamespaceOffloadingList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &namespaceOffloadings)); err != nil {
		return err
	}
	for i := range namespaceOffloadings.Items {
		offloading := &namespaceOffloadings.Items[i]
		errMap.namespaces = append(errMap.namespaces, offloading.Namespace)
	}

	// Search for ResourceSlice resources
	var resourceSlices authv1alpha1.ResourceSliceList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &resourceSlices)); err != nil {
		return err
	}
	for i := range resourceSlices.Items {
		errMap.authentication = addResourceToErrMap(&resourceSlices.Items[i], &errMap, errMap.authentication, &foreignClusterList)
	}

	// Search for Configuration resources
	var configurations networkingv1alpha1.ConfigurationList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &configurations)); err != nil {
		return err
	}
	for i := range configurations.Items {
		errMap.networking = addResourceToErrMap(&configurations.Items[i], &errMap, errMap.networking, &foreignClusterList)
	}

	// Search for IP resources
	var ips ipamv1alpha1.IPList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &ips)); err != nil {
		return err
	}
	for i := range ips.Items {
		// These IPs will be handled by the uninstaller job
		if ipsutils.IsAPIServerIP(&ips.Items[i]) {
			continue
		}

		if len(ips.Items[i].GetFinalizers()) > 0 {
			errMap.networking = addResourceToErrMap(&ips.Items[i], &errMap, errMap.networking, &foreignClusterList)
		}
	}

	// Search for Network resources
	var networks ipamv1alpha1.NetworkList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &networks)); err != nil {
		return err
	}
	for i := range networks.Items {
		// These networks will be handled by the uninstaller job
		if ipamutils.IsExternalCIDR(&networks.Items[i]) || ipamutils.IsInternalCIDR(&networks.Items[i]) {
			continue
		}
		if len(networks.Items[i].GetFinalizers()) > 0 {
			errMap.networking = addResourceToErrMap(&networks.Items[i], &errMap, errMap.networking, &foreignClusterList)
		}
	}

	return errMap.getError()
}

func addResourceToErrMap(obj client.Object, errMap *errorMap, errList []string, foreignClusters *discoveryv1alpha1.ForeignClusterList) []string {
	// Check if object is a resource associated with a remote cluster
	clusterID, found := getRemoteClusterID(obj, foreignClusters)
	if found {
		if !slices.Contains(errList, string(clusterID)) {
			errList = append(errList, string(clusterID))
		}
		return errList
	}

	// If resource is not associated to any cluster, add the object to the generic error list
	addGenericToErrMap(obj, errMap)

	return errList
}

func addGenericToErrMap(obj client.Object, errMap *errorMap) {
	name := fmt.Sprintf("%s: %s/%s", obj.GetObjectKind().GroupVersionKind().GroupKind(), obj.GetNamespace(), obj.GetName())
	if !slices.Contains(errMap.generic, name) {
		errMap.generic = append(errMap.generic, name)
	}
}

func getRemoteClusterID(obj client.Object, foreignClusters *discoveryv1alpha1.ForeignClusterList) (discoveryv1alpha1.ClusterID, bool) {
	v, ok := obj.GetLabels()[consts.RemoteClusterID]
	if ok && v != "" && foreignClusters != nil {
		remoteID := discoveryv1alpha1.ClusterID(v)
		for i := range foreignClusters.Items {
			if foreignClusters.Items[i].Spec.ClusterID == remoteID {
				return foreignClusters.Items[i].Spec.ClusterID, true
			}
		}
	}
	return "", false
}
