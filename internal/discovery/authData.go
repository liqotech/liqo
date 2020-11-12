package discovery

import (
	"errors"
	"github.com/grandcat/zeroconf"
	"k8s.io/klog"
	"net"
	"sync"
)

type AuthData struct {
	address string
	port    int
}

func (authData *AuthData) Get(discovery *DiscoveryCtrl, entry *zeroconf.ServiceEntry) error {
	return authData.Decode(entry)
}

func (authData *AuthData) Decode(entry *zeroconf.ServiceEntry) error {
	authData.port = entry.Port

	ip, err := getReachable(entry.AddrIPv4, entry.Port)
	if err != nil {
		ip, err = getReachable(entry.AddrIPv6, entry.Port)
	}
	if err != nil {
		klog.Error(err)
		return err
	}

	authData.address = ip.String()
	return nil
}

func getReachable(ips []net.IP, port int) (*net.IP, error) {
	resChan := make(chan int, len(ips))
	defer close(resChan)
	wg := sync.WaitGroup{}
	wg.Add(len(ips))

	// search in an async way for all reachable ips
	for i, ip := range ips {
		go func(ip net.IP, port int, index int, ch chan int) {
			if !ip.IsLoopback() && !ip.IsMulticast() && isReachable(ip.String(), port) {
				ch <- index
			}
			wg.Done()
		}(ip, port, i, resChan)
	}
	wg.Wait()

	// if someone is reachable return its index
	select {
	case i := <-resChan:
		return &ips[i], nil
	default:
		return nil, errors.New("server not reachable")
	}
}

func isReachable(address string, port int) bool {
	//_, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), 500*time.Millisecond)
	//return err == nil
	// TODO: no service is available yet
	return true
}
