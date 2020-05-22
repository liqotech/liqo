package discovery

import (
	"github.com/grandcat/zeroconf"
	"math/rand"
	"net"
	"os"
	"strings"
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
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		// TODO: find smarter way
		// select interfaces with IP addresses not in pod local network
		sel := false
		for _, addr := range addrs {
			if !strings.Contains(addr.String(), "10.") {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
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
