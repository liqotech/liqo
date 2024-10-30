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
)

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch

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
	// List all networks present in the cluster.
	nets, err := listNetworksOnCluster(ctx, lipam.Client)
	if err != nil {
		return err
	}

	// Initialize the networks.
	for _, net := range nets {
		if err := lipam.reserveNetwork(net); err != nil {
			klog.Errorf("Failed to reserve network %q: %v", net, err)
			return err
		}
	}

	return nil
}

func (lipam *LiqoIPAM) initializeIPs(ctx context.Context) error {
	// List all IPs present in the cluster.
	ips, err := listIPsOnCluster(ctx, lipam.Client)
	if err != nil {
		return err
	}

	// Initialize the IPs.
	for _, ip := range ips {
		if err := lipam.reserveIP(ip); err != nil {
			klog.Errorf("Failed to reserve IP %q (network %q): %v", ip.ip, ip.cidr, err)
			return err
		}
	}

	return nil
}
