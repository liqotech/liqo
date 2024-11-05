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
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	klog "k8s.io/klog/v2"
)

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch

func (lipam *LiqoIPAM) sync(ctx context.Context, syncFrequency time.Duration) {
	err := wait.PollUntilContextCancel(ctx, syncFrequency, false,
		func(ctx context.Context) (done bool, err error) {
			klog.Info("Started IPAM cache sync routine")
			now := time.Now()
			// networks created before this threshold will be removed from the cache if they are not present in the cluster.
			expiredThreshold := now.Add(-syncFrequency)

			// Sync networks.
			if err := lipam.syncNetworks(ctx, expiredThreshold); err != nil {
				return false, err
			}

			// Sync IPs.
			if err := lipam.syncIPs(ctx, expiredThreshold); err != nil {
				return false, err
			}

			klog.Info("Completed IPAM cache sync routine")
			return false, nil
		})
	if err != nil {
		klog.Errorf("IPAM cache sync routine failed: %v", err)
		os.Exit(1)
	}
}

func (lipam *LiqoIPAM) syncNetworks(ctx context.Context, expiredThreshold time.Time) error {
	// List all networks present in the cluster.
	clusterNetworks, err := listNetworksOnCluster(ctx, lipam.Client)
	if err != nil {
		return err
	}

	// Create the set of networks present in the cluster (for faster lookup later).
	setClusterNetworks := make(map[string]struct{})

	// Add networks that are present in the cluster but not in the cache.
	for _, net := range clusterNetworks {
		if _, inCache := lipam.cacheNetworks[net]; !inCache {
			if err := lipam.reserveNetwork(net); err != nil {
				return err
			}
		}
		setClusterNetworks[net] = struct{}{} // add network to the set
	}

	// Remove networks that are present in the cache but not in the cluster, and were added before the threshold.
	for key := range lipam.cacheNetworks {
		if _, inCluster := setClusterNetworks[key]; !inCluster && lipam.cacheNetworks[key].creationTimestamp.Before(expiredThreshold) {
			lipam.freeNetwork(lipam.cacheNetworks[key].cidr)
		}
	}

	return nil
}

func (lipam *LiqoIPAM) syncIPs(ctx context.Context, expiredThreshold time.Time) error {
	// List all IPs present in the cluster.
	ips, err := listIPsOnCluster(ctx, lipam.Client)
	if err != nil {
		return err
	}

	// Create the set of IPs present in the cluster (for faster lookup later).
	setClusterIPs := make(map[string]struct{})

	// Add IPs that are present in the cluster but not in the cache.
	for _, ip := range ips {
		if _, inCache := lipam.cacheIPs[ip.String()]; !inCache {
			if err := lipam.reserveIP(ip); err != nil {
				return err
			}
		}
		setClusterIPs[ip.String()] = struct{}{} // add IP to the set
	}

	// Remove IPs that are present in the cache but not in the cluster, and were added before the threshold.
	for key := range lipam.cacheIPs {
		if _, inCluster := setClusterIPs[key]; !inCluster && lipam.cacheIPs[key].creationTimestamp.Before(expiredThreshold) {
			lipam.freeIP(lipam.cacheIPs[key].ipCidr)
		}
	}

	return nil
}
