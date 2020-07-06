package discovery

import (
	"context"
	"errors"
	"github.com/grandcat/zeroconf"
	"k8s.io/klog"
	"math"
	"net"
	"os"
	"strconv"
	"time"
)

func (discovery *DiscoveryCtrl) StartResolver() {
	for range time.Tick(time.Second * time.Duration(discovery.Config.UpdateTime)) {
		if discovery.Config.EnableDiscovery {
			discovery.Resolve(discovery.Config.Service, discovery.Config.Domain, int(math.Max(float64(discovery.Config.WaitTime), 1)), nil)
		}
	}
}

func (discovery *DiscoveryCtrl) Resolve(service string, domain string, waitTime int, testRes *[]*TxtData) {
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		if testRes != nil {
			*testRes = discovery.getTxts(results, false)
		} else {
			res := discovery.getTxts(results, true)
			discovery.UpdateForeign(res, nil)
		}
	}(entries)

	var ctx context.Context
	var cancel context.CancelFunc
	if waitTime > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*time.Duration(waitTime))
		defer cancel()
	} else {
		ctx = context.Background()
	}

	err = resolver.Browse(ctx, service, domain, entries)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	<-ctx.Done()
}

func (discovery *DiscoveryCtrl) getTxts(results <-chan *zeroconf.ServiceEntry, onlyForeign bool) []*TxtData {
	var res []*TxtData
	for entry := range results {
		if discovery.isForeign(entry.AddrIPv4) || !onlyForeign {
			ip, err := getEntryIP(entry)
			if err != nil {
				klog.Error(err, err.Error())
				continue
			}
			if txtData, err := Decode(ip, strconv.Itoa(entry.Port), entry.Text); err == nil {
				res = append(res, txtData)
			} else {
				klog.Error(err, err.Error())
			}
		}
	}
	return res
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

func (discovery *DiscoveryCtrl) isForeign(foreignIps []net.IP) bool {
	myIps := discovery.getIPs()
	for _, fIp := range foreignIps {
		if myIps[fIp.String()] {
			return false
		}
		klog.Info("Received packet from " + fIp.String())
	}
	return true
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

func getEntryIP(entry *zeroconf.ServiceEntry) (string, error) {
	if len(entry.AddrIPv4) > 0 {
		return entry.AddrIPv4[0].String(), nil
	}
	if len(entry.AddrIPv6) > 0 {
		return entry.AddrIPv6[0].String(), nil
	}
	return "", errors.New("no IP found")
}
