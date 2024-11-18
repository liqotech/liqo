package proxy

import (
	"bufio"
	"net"
	"net/http"
	"time"
	"k8s.io/klog/v2"
)

func (p *Proxy) handleConnect(c net.Conn) {
	br := bufio.NewReader(c)
	req, err := http.ReadRequest(br)
	if err != nil {
		klog.Errorf("error reading request: %v", err)
		return
	}

	if req.Method != http.MethodConnect {
		response := &http.Response{
			StatusCode: http.StatusMethodNotAllowed,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		response.Write(c)
		c.Close()
		return
	}

	if !p.isAllowed(req.URL.Host) {
		klog.Infof("host %s is not allowed", req.URL.Host)

		response := &http.Response{
			StatusCode: http.StatusForbidden,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		response.Write(c)
		return
	}

	klog.Infof("handling CONNECT to %s", req.URL.Host)

	response := &http.Response{
		StatusCode: 200,
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	response.Write(c)

	destConn, err := net.DialTimeout("tcp", req.URL.Host, 30*time.Second)
	if err != nil {
		response := &http.Response{
			StatusCode: http.StatusRequestTimeout,
			ProtoMajor: 1,
			ProtoMinor: 1,
		}
		response.Write(c)
		return
	}

	go transfer(destConn, c)
	go transfer(c, destConn)
}
