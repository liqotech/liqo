package discovery

import (
	"context"
	"net"
	"os"
	"reflect"
	"time"

	"github.com/grandcat/zeroconf"
	"k8s.io/klog"

	"github.com/liqotech/liqo/internal/discovery/utils"
	"github.com/liqotech/liqo/pkg/auth"
)

func (discovery *Controller) startResolver(stopChan <-chan bool) {
	for {
		if discovery.Config.EnableDiscovery {
			ctx, cancel := context.WithCancel(context.TODO())
			go discovery.resolve(ctx, discovery.Config.AuthService, discovery.Config.Domain, nil)
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
					dData.ClusterInfo, err = discovery.getClusterInfo(defaultInsecureSkipTLSVerify, dData.AuthData)
					if err != nil {
						klog.Error(err)
						continue
					}
					if dData.ClusterInfo.ClusterID == discovery.LocalClusterID.GetClusterID() || dData.ClusterInfo.ClusterID == "" {
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

func (discovery *Controller) getClusterInfo(insecureSkipTLSVerify bool, authData *AuthData) (*auth.ClusterInfo, error) {
	ids, err := utils.GetClusterInfo(insecureSkipTLSVerify, authData.getURL())
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
