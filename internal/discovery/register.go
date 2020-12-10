package discovery

import (
	"context"
	"errors"
	"fmt"
	"github.com/grandcat/zeroconf"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
	"net"
)

const (
	authServiceName = "auth-service"
)

func (discovery *DiscoveryCtrl) Register() {
	if discovery.Config.EnableAdvertisement {
		authPort, err := discovery.getAuthServicePort()
		if err != nil {
			klog.Error(err)
			return
		}

		var ttl = discovery.Config.Ttl
		discovery.serverMux.Lock()
		discovery.mdnsServerAuth, err = zeroconf.Register(discovery.ClusterId.GetClusterID(), discovery.Config.AuthService, discovery.Config.Domain, authPort, nil, discovery.getInterfaces(), ttl)
		discovery.serverMux.Unlock()
		if err != nil {
			klog.Error(err)
			return
		}
		defer discovery.shutdownServer()
		<-discovery.stopMDNS
	}
}

func (discovery *DiscoveryCtrl) shutdownServer() {
	discovery.serverMux.Lock()
	defer discovery.serverMux.Unlock()
	discovery.mdnsServerAuth.Shutdown()
}

// get the NodePort of AuthService
func (discovery *DiscoveryCtrl) getAuthServicePort() (int, error) {
	svc, err := discovery.crdClient.Client().CoreV1().Services(discovery.Namespace).Get(context.TODO(), authServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return 0, err
	}

	if svc.Spec.Type != v1.ServiceTypeNodePort {
		err = fmt.Errorf("this service has not %s type", v1.ServiceTypeNodePort)
		klog.Error(err)
		return 0, err
	}
	if len(svc.Spec.Ports) == 0 || svc.Spec.Ports[0].NodePort == 0 {
		err = errors.New("this service has no nodePort")
		klog.Error(err)
		return 0, err
	}
	return int(svc.Spec.Ports[0].NodePort), nil
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
	nodes, err := discovery.crdClient.Client().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	res := make([]*net.IPNet, 0, len(nodes.Items))
	for _, n := range nodes.Items {
		_, ipnet, err := net.ParseCIDR(n.Spec.PodCIDR)
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
