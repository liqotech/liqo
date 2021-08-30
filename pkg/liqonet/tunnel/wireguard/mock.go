package wireguard

import (
	"fmt"
	"net"
)

const (
	ipv4Literal = "10.1.1.1"
	ipv4Dns     = "ipv4.liqodns.resolver"
	ipv6Literal = "2a00:1450:4001:831::200e"
	ipv6Dns     = "ipv6.liqodns.resolver"
)

func addressResolverMock(network, address string) (*net.IPAddr, error) {
	ipv4Addr := net.ParseIP(ipv4Literal)
	ipv4Map := map[string]net.IP{
		ipv4Literal: ipv4Addr,
		ipv4Dns:     ipv4Addr,
	}
	ipv6Addr := net.ParseIP(ipv6Literal)
	ipv6Map := map[string]net.IP{
		ipv6Literal: ipv6Addr,
		ipv6Dns:     ipv6Addr,
	}
	switch network {
	case "ip4":
		val, found := ipv4Map[address]
		if found {
			return &net.IPAddr{
				IP:   val,
				Zone: "",
			}, nil
		}
		return nil, fmt.Errorf("ip not found")
	case "ip6":
		val, found := ipv6Map[address]
		if found {
			return &net.IPAddr{
				IP:   val,
				Zone: "",
			}, nil
		}
		return nil, fmt.Errorf("ip not found")
	default:
		return nil, fmt.Errorf("ip not found")
	}
}
