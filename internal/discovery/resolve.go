package discovery

import (
	"context"
	"github.com/grandcat/zeroconf"
	"k8s.io/klog"
	"net"
	"os"
	"time"
)

func (discovery *DiscoveryCtrl) StartResolver(stopChan <-chan bool) {
	for {
		if discovery.Config.EnableDiscovery {
			discovery.Resolve(context.TODO(), discovery.Config.Service, discovery.Config.Domain, stopChan, nil)
		} else {
			break
		}
	}
}

func (discovery *DiscoveryCtrl) Resolve(ctx context.Context, service string, domain string, stopChan <-chan bool, resultChan chan *TxtData) {
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	entries := make(chan *zeroconf.ServiceEntry, 10)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			data, err := discovery.getTxt(entry)
			if err != nil {
				klog.Error(err)
				continue
			}
			if resultChan != nil {
				resultChan <- data
			}
			if data != nil {
				// it is not a local cluster
				klog.V(4).Infof("FC data: %v", data)
				discovery.UpdateForeignLAN(data)
			}
		}
	}(entries)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err = resolver.Browse(ctx, service, domain, entries)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	select {
	case <-stopChan:
		return
	case <-time.NewTimer(time.Duration(discovery.resolveContextRefreshTime) * time.Minute).C:
		return
	case <-ctx.Done():
		return
	}
}

func (discovery *DiscoveryCtrl) getTxt(entry *zeroconf.ServiceEntry) (*TxtData, error) {
	if discovery.isForeign(entry.AddrIPv4) {
		txtData, err := Decode("", "", entry.Text)
		if err != nil {
			klog.Error(err)
			return nil, err
		}
		klog.V(4).Infof("Remote cluster found at %s", txtData.ApiUrl)
		txtData.Ttl = entry.TTL
		return txtData, nil
	}
	return nil, nil
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
