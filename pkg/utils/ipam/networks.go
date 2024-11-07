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

package ipam

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	liqogetters "github.com/liqotech/liqo/pkg/utils/getters"
)

// GetPodCIDR retrieves the podCIDR of the local cluster.
func GetPodCIDR(ctx context.Context, cl client.Client) (string, error) {
	nw, err := liqogetters.GetUniqueNetworkByLabel(ctx, cl, labels.SelectorFromSet(map[string]string{
		consts.NetworkTypeLabelKey: string(consts.NetworkTypePodCIDR),
	}))
	if err != nil {
		return "", err
	}

	return nw.Spec.CIDR.String(), nil
}

// GetServiceCIDR retrieves the serviceCIDR of the local cluster.
func GetServiceCIDR(ctx context.Context, cl client.Client) (string, error) {
	nw, err := liqogetters.GetUniqueNetworkByLabel(ctx, cl, labels.SelectorFromSet(map[string]string{
		consts.NetworkTypeLabelKey: string(consts.NetworkTypeServiceCIDR),
	}))
	if err != nil {
		return "", err
	}

	return nw.Spec.CIDR.String(), nil
}

// GetExternalCIDR retrieves the externalCIDR of the local cluster.
func GetExternalCIDR(ctx context.Context, cl client.Client) (string, error) {
	nw, err := liqogetters.GetUniqueNetworkByLabel(ctx, cl, labels.SelectorFromSet(map[string]string{
		consts.NetworkTypeLabelKey: string(consts.NetworkTypeExternalCIDR),
	}))
	if err != nil {
		return "", err
	}

	if nw.Status.CIDR == "" {
		return "", fmt.Errorf("the external CIDR is not yet configured: missing status on the Network resource")
	}

	return nw.Status.CIDR.String(), nil
}

// GetInternalCIDR retrieves the internalCIDR of the local cluster.
func GetInternalCIDR(ctx context.Context, cl client.Client) (string, error) {
	nw, err := liqogetters.GetUniqueNetworkByLabel(ctx, cl, labels.SelectorFromSet(map[string]string{
		consts.NetworkTypeLabelKey: string(consts.NetworkTypeInternalCIDR),
	}))
	if err != nil {
		return "", err
	}

	if nw.Status.CIDR == "" {
		return "", fmt.Errorf("the internal CIDR is not yet configured: missing status on the Network resource")
	}

	return nw.Status.CIDR.String(), nil
}

// GetReservedSubnets retrieves the reserved subnets of the local cluster.
func GetReservedSubnets(ctx context.Context, cl client.Client) ([]string, error) {
	var reservedSubnets []string

	networks, err := liqogetters.GetNetworksByLabel(ctx, cl, labels.SelectorFromSet(map[string]string{
		consts.NetworkTypeLabelKey: string(consts.NetworkTypeReserved),
	}))
	if err != nil {
		return nil, err
	}

	for i := range networks.Items {
		reservedSubnets = append(reservedSubnets, networks.Items[i].Spec.CIDR.String())
	}

	return reservedSubnets, nil
}

// NetworkNotRemapped returns whether the given Network does not need CIDR remapping.
func NetworkNotRemapped(nw *ipamv1alpha1.Network) bool {
	if nw.Labels == nil {
		return false
	}
	value, ok := nw.Labels[consts.NetworkNotRemappedLabelKey]
	return ok && !strings.EqualFold(value, "false")
}

// IsPodCIDR returns whether the given Network is of type PodCIDR.
func IsPodCIDR(nw *ipamv1alpha1.Network) bool {
	if nw.Labels == nil {
		return false
	}
	nwType, ok := nw.Labels[consts.NetworkTypeLabelKey]
	return ok && nwType == string(consts.NetworkTypePodCIDR)
}

// IsServiceCIDR returns whether the given Network is of type ServiceCIDR.
func IsServiceCIDR(nw *ipamv1alpha1.Network) bool {
	if nw.Labels == nil {
		return false
	}
	nwType, ok := nw.Labels[consts.NetworkTypeLabelKey]
	return ok && nwType == string(consts.NetworkTypeServiceCIDR)
}

// IsExternalCIDR returns whether the given Network is of type ExternalCIDR.
func IsExternalCIDR(nw *ipamv1alpha1.Network) bool {
	if nw.Labels == nil {
		return false
	}
	nwType, ok := nw.Labels[consts.NetworkTypeLabelKey]
	return ok && nwType == string(consts.NetworkTypeExternalCIDR)
}

// IsInternalCIDR returns whether the given Network is of type InternalCIDR.
func IsInternalCIDR(nw *ipamv1alpha1.Network) bool {
	if nw.Labels == nil {
		return false
	}
	nwType, ok := nw.Labels[consts.NetworkTypeLabelKey]
	return ok && nwType == string(consts.NetworkTypeInternalCIDR)
}

// IsReservedNetwork returns whether the given Network is of type Reserved.
func IsReservedNetwork(nw *ipamv1alpha1.Network) bool {
	if nw.Labels == nil {
		return false
	}
	nwType, ok := nw.Labels[consts.NetworkTypeLabelKey]
	return ok && nwType == string(consts.NetworkTypeReserved)
}

// CreateNetwork creates a Network resource with the given name and CIDR.
// NeedRemapping indicates whether the Network needs CIDR remapping from IPAM.
// NetworkType indicates the type of the Network (leave empty to not set the type).
func CreateNetwork(ctx context.Context, cl client.Client, name, namespace, cidr string, needRemapping bool, networkType *consts.NetworkType) error {
	network := &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, cl, network, func() error {
		if network.Labels == nil {
			network.Labels = map[string]string{}
		}
		if !needRemapping {
			network.Labels[consts.NetworkNotRemappedLabelKey] = consts.NetworkNotRemappedLabelValue
		}
		if networkType != nil {
			network.Labels[consts.NetworkTypeLabelKey] = string(*networkType)
		}

		network.Spec = ipamv1alpha1.NetworkSpec{
			CIDR: networkingv1beta1.CIDR(cidr),
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
