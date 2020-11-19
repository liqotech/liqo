package discovery

import (
	"context"
	"github.com/grandcat/zeroconf"
	"k8s.io/klog"
	"net"
	"os"
	"reflect"
	"time"
)

func (discovery *DiscoveryCtrl) StartResolver(stopChan <-chan bool) {
	for {
		if discovery.Config.EnableDiscovery {
			ctx, cancel := context.WithCancel(context.TODO())
			go discovery.Resolve(ctx, discovery.Config.Service, discovery.Config.Domain, nil, false)
			go discovery.Resolve(ctx, discovery.Config.AuthService, discovery.Config.Domain, nil, true)
			select {
			case <-stopChan:
				cancel()
			case <-time.NewTimer(time.Duration(discovery.resolveContextRefreshTime) * time.Minute).C:
				cancel()
			}
		} else {
			break
		}
	}
}

func (discovery *DiscoveryCtrl) Resolve(ctx context.Context, service string, domain string, resultChan chan DiscoverableData, isAuth bool) {
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry, isAuth bool) {
		for entry := range results {
			var data DiscoverableData
			if isAuth {
				data = &AuthData{}
			} else {
				data = &TxtData{}
			}
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
					if dData.TxtData.ID == discovery.ClusterId.GetClusterID() || dData.TxtData.ID == "" {
						continue
					}
					klog.V(4).Infof("update %s", entry.Instance)
					discovery.UpdateForeignLAN(dData)
					resolvedData.delete(entry.Instance)
				}
			}
		}
	}(entries, isAuth)

	err = resolver.Browse(ctx, service, domain, entries)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	<-ctx.Done()
}

func (discovery *DiscoveryCtrl) getIPs() map[string]bool {
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

// a cluster is considered as foreign if it has at least one IP different from our IPs
func (discovery *DiscoveryCtrl) isForeign(foreignIps []net.IP) bool {
	myIps := discovery.getIPs()
	for _, fIp := range foreignIps {
		if !myIps[fIp.String()] {
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
