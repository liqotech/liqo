package proxy

import (
	"context"
	"fmt"
	"net"
	"strings"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ manager.Runnable = &Proxy{}

type Proxy struct {
	AllowedHosts []string
	Port         int
}

func New(allowedHosts string, port int) *Proxy {
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
		Port:         port,
	}
}

func (p *Proxy) Start(ctx context.Context) error {
	klog.Infof("proxy listening on port %d", p.Port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", p.Port))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

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
		if host == allowedHost {
			return true
		}
	}
	return false
}
