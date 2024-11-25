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

package mapping

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// EnforceAPIServerIPRemapping creates or updates the IP resource for the API server IP remapping.
func EnforceAPIServerIPRemapping(ctx context.Context, cl client.Client, liqoNamespace string) error {
	var k8sSvc corev1.Service
	if err := cl.Get(ctx, client.ObjectKey{
		Name:      "kubernetes",
		Namespace: "default",
	}, &k8sSvc); err != nil {
		return fmt.Errorf("unable to get the kubernetes service: %w", err)
	}

	ip := &ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.IPTypeAPIServer,
			Namespace: liqoNamespace,
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, cl, ip, func() error {
		if ip.Labels == nil {
			ip.Labels = map[string]string{}
		}

		ip.Labels[consts.IPTypeLabelKey] = consts.IPTypeAPIServer

		ip.Spec.IP = networkingv1beta1.IP(k8sSvc.Spec.ClusterIP)

		return nil
	}); err != nil {
		return fmt.Errorf("unable to create or update the IP %q: %w", ip.Name, err)
	}

	return nil
}

// EnforceAPIServerProxyIPRemapping creates or updates the IP resource for the API server proxy IP remapping.
func EnforceAPIServerProxyIPRemapping(ctx context.Context, cl client.Client, liqoNamespace string) error {
	var svcList corev1.ServiceList
	if err := cl.List(ctx, &svcList, client.InNamespace(liqoNamespace), client.MatchingLabels{
		consts.K8sAppNameKey: "proxy",
	}); err != nil {
		return fmt.Errorf("unable to get the proxy service: %w", err)
	}

	var proxySvc corev1.Service
	switch len(svcList.Items) {
	case 0:
		return fmt.Errorf("no proxy service found")
	case 1:
		proxySvc = svcList.Items[0]
	default:
		return fmt.Errorf("multiple proxy services found")
	}

	if proxySvc.Spec.ClusterIP == "" {
		return fmt.Errorf("the proxy service has no cluster IP")
	}

	ip := &ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      consts.IPTypeAPIServerProxy,
			Namespace: liqoNamespace,
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, cl, ip, func() error {
		if ip.Labels == nil {
			ip.Labels = map[string]string{}
		}

		ip.Labels[consts.IPTypeLabelKey] = consts.IPTypeAPIServerProxy

		ip.Spec.IP = networkingv1beta1.IP(proxySvc.Spec.ClusterIP)

		return nil
	}); err != nil {
		return fmt.Errorf("unable to create or update the IP %q: %w", ip.Name, err)
	}

	return nil
}

// MapAddress maps the address with the network configuration of the cluster.
func MapAddress(ctx context.Context, cl client.Client,
	clusterID liqov1beta1.ClusterID, address string) (string, error) {
	cfg, err := getters.GetConfigurationByClusterID(ctx, cl, clusterID)
	if err != nil {
		return "", err
	}

	return MapAddressWithConfiguration(cfg, address)
}

// MapAddressWithConfiguration maps the address with the network configuration of the cluster.
func MapAddressWithConfiguration(cfg *networkingv1beta1.Configuration, address string) (string, error) {
	var (
		podnet, podnetMapped, extnet, extnetMapped *net.IPNet
		podNetMaskLen, extNetMaskLen               int
		err                                        error
	)

	podNeedsRemap := cfg.Spec.Remote.CIDR.Pod.String() != cfg.Status.Remote.CIDR.Pod.String()
	extNeedsRemap := cfg.Spec.Remote.CIDR.External.String() != cfg.Status.Remote.CIDR.External.String()

	_, podnet, err = net.ParseCIDR(cfg.Spec.Remote.CIDR.Pod.String())
	if err != nil {
		return "", err
	}
	if podNeedsRemap {
		_, podnetMapped, err = net.ParseCIDR(cfg.Status.Remote.CIDR.Pod.String())
		if err != nil {
			return "", err
		}
		podNetMaskLen, _ = podnetMapped.Mask.Size()
	}

	_, extnet, err = net.ParseCIDR(cfg.Spec.Remote.CIDR.External.String())
	if err != nil {
		return "", err
	}
	if extNeedsRemap {
		_, extnetMapped, err = net.ParseCIDR(cfg.Status.Remote.CIDR.External.String())
		if err != nil {
			return "", err
		}
		extNetMaskLen, _ = extnetMapped.Mask.Size()
	}

	paddr := net.ParseIP(address)
	if podNeedsRemap && podnet.Contains(paddr) {
		return RemapMask(paddr, *podnetMapped, podNetMaskLen).String(), nil
	}
	if extNeedsRemap && extnet.Contains(paddr) {
		return RemapMask(paddr, *extnetMapped, extNetMaskLen).String(), nil
	}

	return address, nil
}

// RemapMask remaps the mask of the address.
// Consider that net.IP is always a slice of 16 bytes (big-endian).
// The mask is a slice of 4 or 16 bytes (big-endian).
func RemapMask(addr net.IP, mask net.IPNet, maskLen int) net.IP {
	maskLenBytes := maskLen / 8
	for i := 0; i < maskLenBytes; i++ {
		// i+(len(addr)-len(mask.IP)) allows to start from the rightmost byte of the address.
		// e.g if addr is ipv4 len(addr) = 16, and mask is ipv4 len(mask.IP) = 4, then we start from addr[12].
		addr[i+(len(addr)-len(mask.IP))] = mask.IP[i]
	}
	return addr
}
