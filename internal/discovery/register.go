package discovery

import (
	"github.com/grandcat/zeroconf"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
)

func (discovery *DiscoveryCtrl) Register() {
	if discovery.Config.EnableAdvertisement {
		txt, err := discovery.GetTxtData().Encode()
		if err != nil {
			discovery.Log.Error(err, err.Error())
			return
		}

		server, err := zeroconf.Register(discovery.Config.Name+"_"+discovery.ClusterId.GetClusterID(), discovery.Config.Service, discovery.Config.Domain, discovery.Config.Port, txt, discovery.getInterfaces())
		if err != nil {
			discovery.Log.Error(err, err.Error())
			return
		}
		discovery.stopMDNS = make(chan bool)
		defer server.Shutdown()
		<-discovery.stopMDNS
	}
}

func (discovery *DiscoveryCtrl) getInterfaces() []net.Interface {
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

func (discovery *DiscoveryCtrl) getPodNets() ([]*net.IPNet, error) {
	client, err := clients.NewK8sClient()
	if err != nil {
		return nil, err
	}
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var res []*net.IPNet
	for _, n := range nodes.Items {
		_, ipnet, err := net.ParseCIDR(n.Spec.PodCIDR)
		if err != nil {
			discovery.Log.Error(err, err.Error())
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
