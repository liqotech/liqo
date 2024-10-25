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

	klog "k8s.io/klog/v2"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch

type ipCidr struct {
	ip   string
	cidr string
}

func (lipam *LiqoIPAM) initialize(ctx context.Context) error {
	if err := lipam.initializeNetworks(ctx); err != nil {
		return err
	}

	if err := lipam.initializeIPs(ctx); err != nil {
		return err
	}

	klog.Info("IPAM initialized")
	return nil
}

func (lipam *LiqoIPAM) initializeNetworks(ctx context.Context) error {
	// Initialize the networks.
	nets, err := lipam.getReservedNetworks(ctx)
	if err != nil {
		return err
	}

	for _, net := range nets {
		if err := lipam.reserveNetwork(net); err != nil {
			klog.Errorf("Failed to reserve network %s: %v", net, err)
			return err
		}
	}

	return nil
}

func (lipam *LiqoIPAM) initializeIPs(ctx context.Context) error {
	// Initialize the IPs.
	ips, err := lipam.getReservedIPs(ctx)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		if err := lipam.reserveIP(ip.ip, ip.cidr); err != nil {
			klog.Errorf("Failed to reserve IP %s in network %s: %v", ip.ip, ip.cidr, err)
			return err
		}
	}

	return nil
}

func (lipam *LiqoIPAM) getReservedNetworks(ctx context.Context) ([]string, error) {
	var nets []string
	var networks ipamv1alpha1.NetworkList
	if err := lipam.Options.Client.List(ctx, &networks); err != nil {
		return nil, err
	}

	for i := range networks.Items {
		net := &networks.Items[i]

		var cidr string
		switch {
		case net.Labels != nil && net.Labels[consts.NetworkNotRemappedLabelKey] == consts.NetworkNotRemappedLabelValue:
			cidr = net.Spec.CIDR.String()
		default:
			cidr = net.Status.CIDR.String()
		}
		if cidr == "" {
			klog.Warningf("Network %s has no CIDR", net.Name)
			continue
		}

		nets = append(nets, cidr)
	}

	return nets, nil
}

func (lipam *LiqoIPAM) getReservedIPs(ctx context.Context) ([]ipCidr, error) {
	var ips []ipCidr
	var ipList ipamv1alpha1.IPList
	if err := lipam.Options.Client.List(ctx, &ipList); err != nil {
		return nil, err
	}

	for i := range ipList.Items {
		ip := &ipList.Items[i]

		address := ip.Status.IP.String()
		if address == "" {
			klog.Warningf("IP %s has no address", ip.Name)
			continue
		}

		cidr := ip.Status.CIDR.String()
		if cidr == "" {
			klog.Warningf("IP %s has no CIDR", ip.Name)
			continue
		}

		ips = append(ips, ipCidr{ip: address, cidr: cidr})
	}

	return ips, nil
}

func (lipam *LiqoIPAM) reserveNetwork(cidr string) error {
	// TODO: Reserve the network.
	klog.Infof("Reserved network %s", cidr)
	return nil
}

func (lipam *LiqoIPAM) reserveIP(ip, cidr string) error {
	// TODO: Reserve the IP.
	klog.Infof("Reserved IP %s in network %s", ip, cidr)
	return nil
}
