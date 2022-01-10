// Copyright 2019-2022 The Liqo Authors
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

package discovery

import (
	"context"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/grandcat/zeroconf"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/discoverymanager/utils"
)

func (discovery *Controller) startResolver(ctx context.Context) {
	for {
		ctx, cancel := context.WithCancel(ctx)
		go discovery.resolve(ctx, discovery.mdnsConfig.Service, discovery.mdnsConfig.Domain, nil)
		select {
		case <-ctx.Done():
			cancel()
		case <-time.After(discovery.mdnsConfig.ResolveRefreshTime):
			cancel()
		}
	}
}

func (discovery *Controller) resolve(ctx context.Context, service, domain string, resultChan chan discoverableData) {
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			var data discoverableData = &AuthData{}
			err := data.Get(discovery, entry)
			if err != nil {
				continue
			}
			if resultChan != nil {
				resultChan <- data
			}
			if !reflect.ValueOf(data).IsNil() && data.IsComplete() {
				// it is not a local cluster
				klog.V(4).Infof("FC data: %v", data)
				resolvedData.add(entry.Instance, data)
				if resolvedData.isComplete(entry.Instance) {
					klog.V(4).Infof("%s is complete", entry.Instance)
					dData, err := resolvedData.get(entry.Instance)
					if err != nil {
						klog.Error(err)
						continue
					}
					dData.ClusterInfo, err = discovery.getClusterInfo(ctx, dData.AuthData)
					if err != nil {
						klog.Error(err)
						continue
					}
					if dData.ClusterInfo.ClusterID == discovery.LocalCluster.ClusterID || dData.ClusterInfo.ClusterID == "" {
						continue
					}
					klog.V(4).Infof("update %s", entry.Instance)
					discovery.updateForeignLAN(dData)
					resolvedData.delete(entry.Instance)
				}
			}
		}
	}(entries)

	err = resolver.Browse(ctx, service, domain, entries)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	<-ctx.Done()
}

func (discovery *Controller) getClusterInfo(ctx context.Context, authData *AuthData) (*auth.ClusterInfo, error) {
	ids, err := utils.GetClusterInfo(ctx, discovery.insecureTransport, authData.getURL())
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return ids, nil
}

func (discovery *Controller) getIPs() map[string]bool {
	myIps := map[string]bool{}
	ifaces, err := net.Interfaces()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := getIP(addr)
			if ip != nil {
				myIps[ip.String()] = true
			}
		}
	}
	return myIps
}

// a cluster is considered as foreign if it has at least one IP different from our IPs.
func (discovery *Controller) isForeign(foreignIPs []net.IP) bool {
	myIps := discovery.getIPs()
	for _, fIP := range foreignIPs {
		if !myIps[fIP.String()] {
			return true
		}
	}
	return false
}

func getIP(addr net.Addr) net.IP {
	var ip net.IP
	switch v := addr.(type) {
	case *net.IPNet:
		ip = v.IP
	case *net.IPAddr:
		ip = v.IP
	}
	return ip
}
