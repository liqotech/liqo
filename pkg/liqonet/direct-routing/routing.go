package direct_routing

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"k8s.io/klog"
	"net"
	"strings"
)

//for a given ip address the function returns the gateway ip address
func GetNextHop(ip string) (string, int, error) {
	dst := net.ParseIP(ip)
	//first we get all the routing rules from the main routing table
	rules, err := netlink.RouteList(nil, unix.AF_INET)
	if err != nil {
		return "", 0, fmt.Errorf("an error occurred while listing routing rules: %v", err)
	}
	for _, r := range rules {
		if r.Dst != nil && r.Dst.Contains(dst) {
			if r.Gw != nil {
				return r.Gw.String(), 0, nil
			}
			return "", r.LinkIndex, nil
		}
	}
	return "", 0, fmt.Errorf("gateway not found for ip address %s", ip)
}

//for a given ip address the function returns it's link index
func GetLinkIndex(ip string) (int, error) {
	//first we get all the interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return 0, fmt.Errorf("an error occurred while listing interfaces: %v", err)
	}
	for _, i := range interfaces {
		addresses, err := i.Addrs()
		if err != nil {
			klog.Errorf("an error occurred while getting ip addresses for network interface %s: %v", i.Name, err)
		}
		for _, address := range addresses {
			if strings.Contains(address.String(), ip) {
				return i.Index, nil
			}
		}
	}
	return 0, fmt.Errorf("network interface not found for ip address %s", ip)
}

//this function enables the proxy arp for a given network interface
func EnableProxyArp(ifaceName string) error {
	klog.Infof("enabling proxy arp for interface %s", ifaceName)
	proxyArpFilePath := strings.Join([]string{"/proc/sys/net/ipv4/conf/", ifaceName, "/proxy_arp"}, "")
	err := ioutil.WriteFile(proxyArpFilePath, []byte("1"), 0600)
	if err != nil {
		klog.Errorf("an error occurred while writing to file %s: %v", proxyArpFilePath, err)
		return err
	}
	return nil
}
