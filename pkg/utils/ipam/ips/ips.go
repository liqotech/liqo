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

package ips

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/external-network/remapping"
	"github.com/liqotech/liqo/pkg/utils/getters"
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
			Name:      "api-server",
			Namespace: liqoNamespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, cl, ip, func() error {
		if ip.Labels == nil {
			ip.Labels = map[string]string{}
		}

		ip.Labels[remapping.IPCategoryTargetKey] = remapping.IPCategoryTargetValueMapping

		ip.Spec.IP = networkingv1alpha1.IP(k8sSvc.Spec.ClusterIP)

		return nil
	}); err != nil {
		return fmt.Errorf("unable to create or update the IP %q: %w", ip.Name, err)
	}

	return nil
}

// MapAddressWithConfiguration maps the address with the network configuration of the cluster.
func MapAddressWithConfiguration(ctx context.Context, cl client.Client,
	clusterID discoveryv1alpha1.ClusterID, address string) (string, error) {
	cfg, err := getters.GetConfigurationByClusterID(ctx, cl, clusterID)
	if err != nil {
		return "", err
	}

	var (
		podnet, podnetMapped, extnet, extnetMapped *net.IPNet
		podNetMaskLen, extNetMaskLen               int
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
		return remapMask(paddr, *podnetMapped, podNetMaskLen).String(), nil
	}
	if extNeedsRemap && extnet.Contains(paddr) {
		return remapMask(paddr, *extnetMapped, extNetMaskLen).String(), nil
	}

	return address, nil
}

// remapMask remaps the mask of the address.
// Consider that net.IP is always a slice of 16 bytes (big-endian).
// The mask is a slice of 4 or 16 bytes (big-endian).
func remapMask(addr net.IP, mask net.IPNet, maskLen int) net.IP {
	maskLenBytes := maskLen / 8
	for i := 0; i < maskLenBytes; i++ {
		// i+(len(addr)-len(mask.IP)) allows to start from the rightmost byte of the address.
		// e.g if addr is ipv4 len(addr) = 16, and mask is ipv4 len(mask.IP) = 4, then we start from addr[12].
		addr[i+(len(addr)-len(mask.IP))] = mask.IP[i]
	}
	return addr
}
