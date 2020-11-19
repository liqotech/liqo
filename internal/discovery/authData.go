package discovery

import (
	"errors"
	"fmt"
	"github.com/grandcat/zeroconf"
	"k8s.io/klog"
	"net"
	"sync"
	"time"
)

type AuthData struct {
	address string
	port    int
}

func (authData *AuthData) Get(discovery *DiscoveryCtrl, entry *zeroconf.ServiceEntry) error {
	if discovery.isForeign(entry.AddrIPv4) {
		return authData.Decode(entry, discovery.dialTcpTimeout)
	}
	return nil
}

// check if both address and port are correctly set
func (authData *AuthData) IsComplete() bool {
	return authData.address != "" && authData.port > 0
}

// populate the AuthData struct from a DNS entry
// takes as argument the DNS entry and a timeout used to find reachable remote services
func (authData *AuthData) Decode(entry *zeroconf.ServiceEntry, timeout time.Duration) error {
	authData.port = entry.Port

	// checks if there is an IPv4 reachable
	ip, err := getReachable(entry.AddrIPv4, entry.Port, timeout)
	if err != nil {
		// checks if there is an IPv6 reachable
		ip, err = getReachable(entry.AddrIPv6, entry.Port, timeout)
	}
	if err != nil {
		klog.Errorf("%v %v %v", err, entry.AddrIPv4, entry.Port)
		return err
	}

	// use the reachable IP
	// this is the IP that will be contacted to get the required permissions from the remote cluster
	authData.address = ip.String()
	return nil
}

// look for a reachable IP in the ips array
// this is done in an async and parallel way to not to take too much time if the IP list is long
func getReachable(ips []net.IP, port int, timeout time.Duration) (*net.IP, error) {
	resChan := make(chan int, len(ips))
	defer close(resChan)
	wg := sync.WaitGroup{}
	wg.Add(len(ips))

	// search in an async way for all reachable ips
	for i, ip := range ips {
		go func(ip net.IP, port int, index int, ch chan int) {
			if !ip.IsLoopback() && !ip.IsMulticast() && isReachable(ip.String(), port, timeout) {
				ch <- index
			}
			wg.Done()
		}(ip, port, i, resChan)
	}
	wg.Wait()

	// if someone is reachable (the first reachable) return its index
	select {
	case i := <-resChan:
		return &ips[i], nil
	default:
		return nil, errors.New("server not reachable")
	}
}

// check if this address + port is reachable with TCP
// the service is reachable if we are able to establish a TCP connection before the timeout
func isReachable(address string, port int, timeout time.Duration) bool {
	_, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), timeout)
	klog.V(4).Infof("%s:%d %v", address, port, err)
	return err == nil
}
