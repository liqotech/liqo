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
	"time"

	klog "k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
)

type networkInfo struct {
	cidr              string
	creationTimestamp time.Time
}

// reserveNetwork reserves a network, saving it in the cache.
func (lipam *LiqoIPAM) reserveNetwork(cidr string) error {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	if lipam.cacheNetworks == nil {
		lipam.cacheNetworks = make(map[string]networkInfo)
	}
	lipam.cacheNetworks[cidr] = networkInfo{
		cidr:              cidr,
		creationTimestamp: time.Now(),
	}

	klog.Infof("Reserved network %q", cidr)
	return nil
}

// acquireNetwork acquires a network, eventually remapped if conflicts are found.
func (lipam *LiqoIPAM) acquireNetwork(cidr string, immutable bool) (string, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	// TODO: implement real network acquire logic
	_ = immutable
	if lipam.cacheNetworks == nil {
		lipam.cacheNetworks = make(map[string]networkInfo)
	}
	lipam.cacheNetworks[cidr] = networkInfo{
		cidr:              cidr,
		creationTimestamp: time.Now(),
	}

	klog.Infof("Acquired network %q", cidr)
	return cidr, nil
}

// freeNetwork frees a network, removing it from the cache.
func (lipam *LiqoIPAM) freeNetwork(cidr string) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	delete(lipam.cacheNetworks, cidr)
	klog.Infof("Freed network %q", cidr)
}

// isNetworkAvailable checks if a network is available.
func (lipam *LiqoIPAM) isNetworkAvailable(cidr string) bool {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	if lipam.cacheNetworks == nil {
		return true
	}
	_, ok := lipam.cacheNetworks[cidr]

	return ok
}

func listNetworksOnCluster(ctx context.Context, cl client.Client) ([]string, error) {
	var nets []string
	var networks ipamv1alpha1.NetworkList
	if err := cl.List(ctx, &networks); err != nil {
		return nil, err
	}

	for i := range networks.Items {
		net := &networks.Items[i]

		cidr := net.Status.CIDR.String()
		if cidr == "" {
			klog.Warningf("Network %q has no CIDR", net.Name)
			continue
		}

		nets = append(nets, cidr)
	}

	return nets, nil
}
