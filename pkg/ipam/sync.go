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

package ipam

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	klog "k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/utils/maps"
)

// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,verbs=get;list;watch

func (lipam *LiqoIPAM) sync(ctx context.Context, syncFrequency time.Duration) {
	if syncFrequency == 0 {
		klog.Info("IPAM cache sync routine disabled")
		return
	}

	err := wait.PollUntilContextCancel(ctx, syncFrequency, false,
		func(ctx context.Context) (done bool, err error) {
			lipam.mutex.Lock()
			defer lipam.mutex.Unlock()
			klog.V(3).Infof("Started IPAM cache sync routine (grace period: %s)", lipam.opts.SyncGracePeriod)

			// Sync networks.
			if err := lipam.syncNetworks(ctx); err != nil {
				return false, err
			}

			// Sync IPs.
			if err := lipam.syncIPs(ctx); err != nil {
				return false, err
			}

			klog.V(3).Info("Completed IPAM cache sync routine")
			return false, nil
		})
	if err != nil {
		klog.Errorf("IPAM cache sync routine failed: %v", err)
		os.Exit(1)
	}
}

func syncNetworkAcquire(lipam *LiqoIPAM, clusterNetworks map[netip.Prefix]prefixDetails, cachedNetworks map[netip.Prefix]any) error {
	// Add networks that are present in the cluster but not in the cache.
	for clusterNetwork, clusterNetworkDetails := range clusterNetworks {
		if _, ok := cachedNetworks[clusterNetwork]; !ok {
			if _, err := lipam.networkAcquireSpecific(clusterNetwork); err != nil {
				return fmt.Errorf("failed to acquire network %q: %w", clusterNetwork, err)
			}
		}

		if err := lipam.acquirePreallocatedIPs(clusterNetwork, clusterNetworkDetails.preallocated); err != nil {
			return errors.Join(err, lipam.networkRelease(clusterNetwork, 0))
		}
	}

	return nil
}

func isNetworkInCluster(clusterNetworks map[netip.Prefix]prefixDetails, cachedNetwork netip.Prefix) bool {
	_, ok := clusterNetworks[cachedNetwork]
	return ok
}

func syncNetworkFree(lipam *LiqoIPAM, clusterNetworks map[netip.Prefix]prefixDetails, cachedNetworks map[netip.Prefix]any) error {
	// Remove networks that are present in the cache but not in the cluster, and were added before the threshold.
	for cachedNetwork := range cachedNetworks {
		if !isNetworkInCluster(clusterNetworks, cachedNetwork) {
			if err := lipam.networkRelease(cachedNetwork, lipam.opts.SyncGracePeriod); err != nil {
				return fmt.Errorf("failed to free network %q: %w", cachedNetwork.String(), err)
			}
		}
	}
	return nil
}

func (lipam *LiqoIPAM) syncNetworks(ctx context.Context) error {
	// List all networks present in the cluster.
	clusterNetworksMap, err := lipam.listNetworksOnCluster(ctx)
	if err != nil {
		return err
	}

	cachedNetworksMap := maps.SliceToMap(lipam.IpamCore.ListNetworks())

	if err := syncNetworkAcquire(lipam, clusterNetworksMap, cachedNetworksMap); err != nil {
		return fmt.Errorf("failed to acquire network: %w", err)
	}

	if err := syncNetworkFree(lipam, clusterNetworksMap, cachedNetworksMap); err != nil {
		return fmt.Errorf("failed to free network: %w", err)
	}

	return nil
}

func syncIPsAcquire(lipam *LiqoIPAM, clusterIPs, cachedIPs map[netip.Addr]netip.Prefix) error {
	for clusterIP, clusterNetwork := range clusterIPs {
		if _, ok := cachedIPs[clusterIP]; !ok {
			if err := lipam.ipAcquireWithAddr(clusterIP, clusterNetwork); err != nil {
				return fmt.Errorf("failed to acquire IP %q in network %q: %w", clusterIP.String(), clusterNetwork.String(), err)
			}
		}
	}
	return nil
}

func iscachedIPPreallocated(cachedIP netip.Addr, cachedNetwork netip.Prefix, clusterNetworks map[netip.Prefix]prefixDetails) bool {
	details, ok := clusterNetworks[cachedNetwork]
	if !ok {
		return false
	}
	preallocatedIP := cachedNetwork.Addr()
	for i := 0; i < int(details.preallocated); i++ {
		if cachedIP.Compare(preallocatedIP) == 0 {
			return true
		}
		preallocatedIP = preallocatedIP.Next()
	}
	return false
}

func syncIPsFree(lipam *LiqoIPAM, clusterIPs, cachedIPs map[netip.Addr]netip.Prefix, clusterNetworks map[netip.Prefix]prefixDetails) error {
	for cachedIP, cachedNetwork := range cachedIPs {
		if _, ok := clusterIPs[cachedIP]; !ok && !iscachedIPPreallocated(cachedIP, cachedNetwork, clusterNetworks) {
			if err := lipam.ipRelease(cachedIP, cachedNetwork, lipam.opts.SyncGracePeriod); err != nil {
				return fmt.Errorf("failed to free IP %q: %w", cachedIP.String(), err)
			}
		}
	}
	return nil
}

func (lipam *LiqoIPAM) syncIPs(ctx context.Context) error {
	// List all IPs present in the cluster.
	clusterIPsMap, err := lipam.listIPsOnCluster(ctx)
	if err != nil {
		return err
	}

	clusterNetworksMap, err := lipam.listNetworksOnCluster(ctx)
	if err != nil {
		return err
	}

	cachedIPsMap := make(map[netip.Addr]netip.Prefix)
	cachedNetworksList := lipam.IpamCore.ListNetworks()
	for i := range cachedNetworksList {
		cachedIPs, err := lipam.IpamCore.ListIPs(cachedNetworksList[i])
		if err != nil {
			return fmt.Errorf("failed to list IPs in network %q: %w", cachedNetworksList[i].String(), err)
		}
		for j := range cachedIPs {
			cachedIPsMap[cachedIPs[j]] = cachedNetworksList[i]
		}
	}

	if err := syncIPsAcquire(lipam, clusterIPsMap, cachedIPsMap); err != nil {
		return fmt.Errorf("failed to acquire IP: %w", err)
	}

	if err := syncIPsFree(lipam, clusterIPsMap, cachedIPsMap, clusterNetworksMap); err != nil {
		return fmt.Errorf("failed to free IP: %w", err)
	}

	return nil
}
