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
	network
	creationTimestamp time.Time
}

type network struct {
	cidr         string
	preAllocated uint
}

func (n network) String() string {
	return n.cidr
}

// reserveNetwork reserves a network, saving it in the cache.
func (lipam *LiqoIPAM) reserveNetwork(nw network) error {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	// TODO: implement real network reserve logic
	if lipam.cacheNetworks == nil {
		lipam.cacheNetworks = make(map[string]networkInfo)
	}
	lipam.cacheNetworks[nw.String()] = networkInfo{
		network:           nw,
		creationTimestamp: time.Now(),
	}

	klog.Infof("Reserved network %q", nw)
	return nil
}

// acquireNetwork acquires a network, eventually remapped if conflicts are found.
func (lipam *LiqoIPAM) acquireNetwork(cidr string, preAllocated uint, immutable bool) (string, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	// TODO: implement real network acquire logic
	_ = immutable
	if lipam.cacheNetworks == nil {
		lipam.cacheNetworks = make(map[string]networkInfo)
	}
	nw := network{
		cidr:         cidr,
		preAllocated: preAllocated,
	}
	lipam.cacheNetworks[nw.String()] = networkInfo{
		network:           nw,
		creationTimestamp: time.Now(),
	}

	klog.Infof("Acquired network %q", nw.cidr)
	return nw.cidr, nil
}

// freeNetwork frees a network, removing it from the cache.
func (lipam *LiqoIPAM) freeNetwork(nw network) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	// TODO: implement real network free logic
	delete(lipam.cacheNetworks, nw.String())
	klog.Infof("Freed network %q", nw.cidr)
}

// isNetworkAvailable checks if a network is available.
func (lipam *LiqoIPAM) isNetworkAvailable(nw network) bool {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	// TODO: implement real network availability check logic
	if lipam.cacheNetworks == nil {
		return true
	}
	_, ok := lipam.cacheNetworks[nw.String()]

	return ok
}

func listNetworksOnCluster(ctx context.Context, cl client.Client) ([]network, error) {
	var nets []network
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

		nets = append(nets, network{
			cidr:         cidr,
			preAllocated: net.Spec.PreAllocated,
		})
	}

	return nets, nil
}
