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
	"errors"
	"fmt"
	"net"

	"github.com/grandcat/zeroconf"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

func (discovery *Controller) register(ctx context.Context) {
	authPort, err := discovery.getAuthServicePort(ctx)
	if err != nil {
		klog.Error(err)
		return
	}

	discovery.serverMux.Lock()
	discovery.mdnsServerAuth, err = zeroconf.Register(
		discovery.LocalCluster.ClusterID,
		discovery.mdnsConfig.Service,
		discovery.mdnsConfig.Domain,
		authPort, nil, discovery.getInterfaces(),
		uint32(discovery.mdnsConfig.TTL.Seconds()))
	discovery.serverMux.Unlock()
	if err != nil {
		klog.Error(err)
		return
	}
	defer discovery.shutdownServer()
	<-ctx.Done()
}

func (discovery *Controller) shutdownServer() {
	discovery.serverMux.Lock()
	defer discovery.serverMux.Unlock()
	discovery.mdnsServerAuth.Shutdown()
}

// get the NodePort of AuthService.
func (discovery *Controller) getAuthServicePort(ctx context.Context) (int, error) {
	var svc v1.Service
	key := types.NamespacedName{Namespace: discovery.namespace, Name: liqoconst.AuthServiceName}
	if err := discovery.namespacedClient.Get(ctx, key, &svc); err != nil {
		klog.Error(err)
		return 0, err
	}

	if svc.Spec.Type != v1.ServiceTypeNodePort {
		err := fmt.Errorf("this service has not %s type", v1.ServiceTypeNodePort)
		klog.Error(err)
		return 0, err
	}
	if len(svc.Spec.Ports) == 0 || svc.Spec.Ports[0].NodePort == 0 {
		err := errors.New("this service has no nodePort")
		klog.Error(err)
		return 0, err
	}
	return int(svc.Spec.Ports[0].NodePort), nil
}

func (discovery *Controller) getInterfaces() []net.Interface {
	var interfaces []net.Interface
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	podNets, err := discovery.getPodNets()
	if err != nil {
		return nil
	}
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		// select interfaces with IP addresses not in pod local network
		sel := false
		for _, addr := range addrs {
			ip := getIP(addr)
			if !isPod(podNets, ip) {
				if ip != nil && ip.To4() != nil {
					sel = true
				}
			}
		}
		if !sel {
			continue
		}

		if (ifi.Flags & net.FlagUp) == 0 {
			continue
		}
		if (ifi.Flags & net.FlagMulticast) > 0 {
			interfaces = append(interfaces, ifi)
		}
	}
	return interfaces
}

func (discovery *Controller) getPodNets() ([]*net.IPNet, error) {
	var nodes v1.NodeList
	if err := discovery.List(context.TODO(), &nodes); err != nil {
		return nil, err
	}

	res := make([]*net.IPNet, 0, len(nodes.Items))
	for i := range nodes.Items {
		_, ipnet, err := net.ParseCIDR(nodes.Items[i].Spec.PodCIDR)
		if err != nil {
			klog.Error(err, err.Error())
			continue
		}
		res = append(res, ipnet)
	}
	return res, nil
}

func isPod(podNets []*net.IPNet, ip net.IP) bool {
	for _, ipnet := range podNets {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}
