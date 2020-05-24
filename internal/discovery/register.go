package discovery

import (
	"github.com/grandcat/zeroconf"
	"github.com/netgroup-polito/dronev2/internal/discovery/clients"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"net"
	"os"
	"time"
)

var server *zeroconf.Server

func Register(name string, service string, domain string, port int, txt []string) {
	var err error = nil
	// random string needed because equal names are discarded
	server, err = zeroconf.Register(name+"_"+RandomString(8), service, domain, port, txt, GetInterfaces())
	if err != nil {
		Log.Error(err, err.Error())
		os.Exit(1)
	}
	defer server.Shutdown()

	select {}
}

func SetText(txt []string) {
	server.SetText(txt)
}

func RandomString(nChars uint) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, nChars)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GetInterfaces() []net.Interface {
	var interfaces []net.Interface
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	podNets, err := getPodNets()
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

func getPodNets() ([]*net.IPNet, error) {
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
			Log.Error(err, err.Error())
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
