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

package mapping

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
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

		if ip.Annotations == nil {
			ip.Annotations = map[string]string{}
		}
		ip.Annotations[consts.PreinstalledAnnotKey] = consts.PreinstalledAnnotValue

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

		if ip.Annotations == nil {
			ip.Annotations = map[string]string{}
		}
		ip.Annotations[consts.PreinstalledAnnotKey] = consts.PreinstalledAnnotValue

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
	cfg, err := getters.GetConfigurationByClusterID(ctx, cl, clusterID, corev1.NamespaceAll)
	if err != nil {
		return "", err
	}

	return MapAddressWithConfiguration(cfg, address)
}

// MapAddressWithConfiguration maps the address with the network configuration of the cluster.
func MapAddressWithConfiguration(cfg *networkingv1beta1.Configuration, address string) (string, error) {
	var (
		podnet, podnetMapped, extnet, extnetMapped *net.IPNet
		err                                        error
	)

	podNeedsRemap := cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.Pod).String() != cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String()
	extNeedsRemap := cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.External).String() != cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String()

	_, podnet, err = net.ParseCIDR(cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.Pod).String())
	if err != nil {
		return "", err
	}
	if podNeedsRemap {
		_, podnetMapped, err = net.ParseCIDR(cidrutils.GetPrimary(cfg.Status.Remote.CIDR.Pod).String())
		if err != nil {
			return "", err
		}
	}

	_, extnet, err = net.ParseCIDR(cidrutils.GetPrimary(cfg.Spec.Remote.CIDR.External).String())
	if err != nil {
		return "", err
	}
	if extNeedsRemap {
		_, extnetMapped, err = net.ParseCIDR(cidrutils.GetPrimary(cfg.Status.Remote.CIDR.External).String())
		if err != nil {
			return "", err
		}
	}

	paddr := net.ParseIP(address)
	if podNeedsRemap && podnet.Contains(paddr) {
		return RemapMask(paddr, *podnetMapped).String(), nil
	}
	if extNeedsRemap && extnet.Contains(paddr) {
		return RemapMask(paddr, *extnetMapped).String(), nil
	}

	return address, nil
}

// RemapMask take an IP address and a network mask and remap the address to the network.
// This means that the host part of the address is preserved, while the network part is replaced with the one in the mask.
//
// Example:
// addr: 		10.1.0.1
// mask:    	40.32.0.0/10
// result:  	40.1.0.1
// addrBin: 	00001010000000010000000000000001
// maskBin: 	11111111110000000000000000000000
// netBin : 	00101000000000000000000000000000
// hostBin: 	00000000000000010000000000000001
// resultBin : 	00101000000000010000000000000001
//
// addr:		10.255.1.1
// mask:		78.5.78.143/18
// result:		78.5.65.1
// addrBin: 	00001010111111110000000100000001
// maskBin: 	11111111111111111100000000000000
// netBin : 	01001110000001010100000000000000
// hostBin: 	00000000000000000000000100000001
// resultBin:	01001110000001010100000100000001 // nolint: godot // this comment must not end with a period.
func RemapMask(addr net.IP, mask net.IPNet) net.IP {
	switch len(mask.IP) {
	case net.IPv4len:
		addr = addr.To4()

		// Convert addr,mask,net to binary representation
		// Check the comment of the RemapMask function to better understand the values stored in the variables.
		addrBin := binary.BigEndian.Uint32(addr)
		maskBin := binary.BigEndian.Uint32(mask.Mask)
		netBin := binary.BigEndian.Uint32(mask.IP)

		// Calculate the host part of the address
		// We need to invert the mask and apply the AND operation to the address
		// to keep only the host part
		hostBin := addrBin & (^maskBin)

		// Calculate the result
		// We need to apply the OR operation to the network part and the host part
		result := netBin | hostBin

		resultBytes := make([]byte, 4)

		binary.BigEndian.PutUint32(resultBytes, result)

		return resultBytes
	case net.IPv6len:
		// Refer to the IPv4 case for the explanation of the following operations.
		addr = addr.To16()

		addrBin1 := binary.BigEndian.Uint64(addr[:8])
		addrBin2 := binary.BigEndian.Uint64(addr[8:])
		maskBin1 := binary.BigEndian.Uint64(mask.Mask[:8])
		maskBin2 := binary.BigEndian.Uint64(mask.Mask[8:])
		netBin1 := binary.BigEndian.Uint64(mask.IP[:8])
		netBin2 := binary.BigEndian.Uint64(mask.IP[8:])

		hostBin1 := addrBin1 & (^maskBin1)
		hostBin2 := addrBin2 & (^maskBin2)

		result1 := netBin1 | hostBin1
		result2 := netBin2 | hostBin2

		resultBytes := make([]byte, 16)

		binary.BigEndian.PutUint64(resultBytes[:8], result1)
		binary.BigEndian.PutUint64(resultBytes[8:], result2)

		return resultBytes
	}
	return nil
}
