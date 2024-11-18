package proxy

import (
	"fmt"
	"io"
	"net"
	"strings"

	"k8s.io/klog/v2"
)

type Proxy struct {
	AllowedHosts []string
}

func New(allowedHosts string) *Proxy {
	ah := strings.Split(allowedHosts, ",")
	// remove empty strings
	for i := 0; i < len(ah); i++ {
		if ah[i] == "" {
			ah = append(ah[:i], ah[i+1:]...)
			i--
		}
	}

	return &Proxy{
		AllowedHosts: ah,
	}
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func (p *Proxy) SetupProxy(port int) error {
	klog.Infof("proxy listening on port %d", port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			klog.Errorf("error accepting connection: %v", err)
			continue
		}

		go p.handleConnect(conn)
	}
}

func (p *Proxy) isAllowed(host string) bool {
	if len(p.AllowedHosts) == 0 {
		return true
	}

	for _, allowedHost := range p.AllowedHosts {
		klog.Infof("allowed host: %s", allowedHost)
		if host == allowedHost {
			return true
		}
	}
	return false
}
