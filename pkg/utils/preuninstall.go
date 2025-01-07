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

package utils

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/errors"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

// UninstallErrorType is the type of uninstall error.
type UninstallErrorType string

const (
	// GenericUninstallError is an error related to resources that needs to be removed.
	GenericUninstallError = "generic"
	// PendingActivePeerings is an error related peerings still active.
	PendingActivePeerings = "pendingActivePeerings"
	// PendingOffloadedNamespaces is an error related to namespaces still offloaded.
	PendingOffloadedNamespaces = "pendingOffloadedNamespaces"
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

// UninstallError is an error type returned when there are errors during the uninstall process.
type UninstallError struct {
	errorTypes []UninstallErrorType
	message    string
}

// GetErrorTypes returns the type of uninstall error.
func (ue UninstallError) GetErrorTypes() []UninstallErrorType {
	return ue.errorTypes
}

// Error returns the error message.
func (ue UninstallError) Error() string {
	return ue.message
}

func (em *errorMap) getError() error {
	str := ""
	errorTypes := map[UninstallErrorType]bool{}

	if len(em.networking) > 0 {
		str += "\ndisable networking for clusters:\n"
		for _, fc := range em.networking {
			str += fmt.Sprintf("- %s\n", fc)
		}
		errorTypes[PendingActivePeerings] = true
	}

	if len(em.authentication) > 0 {
		str += "\ndisable authentication for clusters:\n"
		for i := range em.authentication {
			str += fmt.Sprintf("- %s\n", em.authentication[i])
		}
		errorTypes[PendingActivePeerings] = true
	}

	if len(em.offloading) > 0 {
		str += "\ndisable offloading for clusters:\n"
		for _, fc := range em.offloading {
			str += fmt.Sprintf("- %s\n", fc)
		}
		errorTypes[PendingActivePeerings] = true
	}

	if len(em.namespaces) > 0 {
		str += "\nunoffload the following namespaces:\n"
		for i := range em.namespaces {
			str += fmt.Sprintf("- %s\n", em.namespaces[i])
		}
		errorTypes[PendingOffloadedNamespaces] = true
	}

	if len(em.generic) > 0 {
		str += "\nremove the following resources:\n"
		for i := range em.generic {
			str += fmt.Sprintf("- %s\n", em.generic[i])
		}
		errorTypes[GenericUninstallError] = true
	}

	if len(errorTypes) > 0 {
		msg := fmt.Sprintf("you should:\n%s", str)
		return UninstallError{
			errorTypes: slices.Collect(maps.Keys(errorTypes)),
			message:    msg,
		}
	}

	return nil
}

// PreUninstall checks if there are resources that need to be removed before uninstalling Liqo.
func PreUninstall(ctx context.Context, cl client.Client) error {
	var foreignClusterList liqov1beta1.ForeignClusterList
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
	var namespaceOffloadings offloadingv1beta1.NamespaceOffloadingList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &namespaceOffloadings)); err != nil {
		return err
	}
	for i := range namespaceOffloadings.Items {
		offloading := &namespaceOffloadings.Items[i]
		errMap.namespaces = append(errMap.namespaces, offloading.Namespace)
	}

	// Search for ResourceSlice resources
	var resourceSlices authv1beta1.ResourceSliceList
	if err := errors.IgnoreNoMatchError(cl.List(ctx, &resourceSlices)); err != nil {
		return err
	}
	for i := range resourceSlices.Items {
		errMap.authentication = addResourceToErrMap(&resourceSlices.Items[i], &errMap, errMap.authentication, &foreignClusterList)
	}

	// Search for Configuration resources
	var configurations networkingv1beta1.ConfigurationList
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
		if ipamutils.IsAPIServerIP(&ips.Items[i]) {
			continue
		}
		if ipamutils.IsAPIServerProxyIP(&ips.Items[i]) {
			continue
		}

		// These IPs were installed at install-time and should not be removed unless liqo is uninstalled.
		if IsPreinstalledResource(&ips.Items[i]) {
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
		// These networks were installed at install-time and should not be removed unless liqo is uninstalled.
		if IsPreinstalledResource(&networks.Items[i]) {
			continue
		}
		if len(networks.Items[i].GetFinalizers()) > 0 {
			errMap.networking = addResourceToErrMap(&networks.Items[i], &errMap, errMap.networking, &foreignClusterList)
		}
	}

	return errMap.getError()
}

// IsPreinstalledResource returns whether the given resource was created at install-time by Liqo.
func IsPreinstalledResource(obj metav1.Object) bool {
	if obj.GetAnnotations() == nil {
		return false
	}
	value, ok := obj.GetAnnotations()[consts.PreinstalledAnnotKey]
	return ok && !strings.EqualFold(value, "false")
}

func addResourceToErrMap(obj client.Object, errMap *errorMap, errList []string, foreignClusters *liqov1beta1.ForeignClusterList) []string {
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

func getRemoteClusterID(obj client.Object, foreignClusters *liqov1beta1.ForeignClusterList) (liqov1beta1.ClusterID, bool) {
	v, ok := obj.GetLabels()[consts.RemoteClusterID]
	if ok && v != "" && foreignClusters != nil {
		remoteID := liqov1beta1.ClusterID(v)
		for i := range foreignClusters.Items {
			if foreignClusters.Items[i].Spec.ClusterID == remoteID {
				return foreignClusters.Items[i].Spec.ClusterID, true
			}
		}
	}
	return "", false
}
