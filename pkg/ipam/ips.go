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

type ipInfo struct {
	ipCidr
	creationTimestamp time.Time
}

type ipCidr struct {
	ip   string
	cidr string
}

func (i ipCidr) String() string {
	return i.ip + "-" + i.cidr
}

// reserveIP reserves an IP, saving it in the cache.
func (lipam *LiqoIPAM) reserveIP(ip ipCidr) error {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	if lipam.cacheIPs == nil {
		lipam.cacheIPs = make(map[string]ipInfo)
	}
	lipam.cacheIPs[ip.String()] = ipInfo{
		ipCidr:            ip,
		creationTimestamp: time.Now(),
	}

	klog.Infof("Reserved IP %q (network %q)", ip.ip, ip.cidr)
	return nil
}

// acquireIP acquires an IP, eventually remapped if conflicts are found.
func (lipam *LiqoIPAM) acquireIP(cidr string) (string, error) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	// TODO: implement real IP acquire logic
	if lipam.cacheIPs == nil {
		lipam.cacheIPs = make(map[string]ipInfo)
	}
	ip := ipCidr{
		ip:   "",
		cidr: cidr,
	}
	lipam.cacheIPs[ip.String()] = ipInfo{
		ipCidr:            ip,
		creationTimestamp: time.Now(),
	}

	klog.Infof("Acquired IP %q (network %q)", ip.ip, ip.cidr)
	return ip.ip, nil
}

// freeIP frees an IP, removing it from the cache.
func (lipam *LiqoIPAM) freeIP(ip ipCidr) {
	lipam.mutex.Lock()
	defer lipam.mutex.Unlock()

	delete(lipam.cacheIPs, ip.String())
	klog.Infof("Freed IP %q (network %q)", ip.ip, ip.cidr)
}

func listIPsOnCluster(ctx context.Context, cl client.Client) ([]ipCidr, error) {
	var ips []ipCidr
	var ipList ipamv1alpha1.IPList
	if err := cl.List(ctx, &ipList); err != nil {
		return nil, err
	}

	for i := range ipList.Items {
		ip := &ipList.Items[i]

		address := ip.Status.IP.String()
		if address == "" {
			klog.Warningf("IP %q has no address", ip.Name)
			continue
		}

		cidr := ip.Status.CIDR.String()
		if cidr == "" {
			klog.Warningf("IP %q has no CIDR", ip.Name)
			continue
		}

		ips = append(ips, ipCidr{ip: address, cidr: cidr})
	}

	return ips, nil
}
