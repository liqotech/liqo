package overlay

import (
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	k8s "k8s.io/client-go/kubernetes"
	"strings"
)

const (
	secretPrefix          = "overlaykeys-"
	WgInterfacename       = "liqo.overlay"
	NetworkPrefix         = "240"
	WgListeningPort       = "51871"
	PubKeyAnnotation      = "net.liqo.io/overlay.pubkey"
	NodeCIDRKeyAnnotation = "net.liqo.io/node.cidr"
)

var (
	wgPort = 51871
	wgMtu  = 1300
)

func CreateInterface(nodeName, namespace, ipAddr string, c *k8s.Clientset, wgc wireguard.Client, nl wireguard.Netlinker) (*wireguard.Wireguard, error) {
	secretName := strings.Join([]string{secretPrefix, nodeName}, "")
	priv, pub, err := wireguard.GetKeys(secretName, namespace, c)
	if err != nil {
		return nil, err
	}
	wgConfig := wireguard.WgConfig{
		Name:      WgInterfacename,
		IPAddress: ipAddr,
		Mtu:       wgMtu,
		Port:      &wgPort,
		PriKey:    &priv,
		PubKey:    &pub,
	}
	wg, err := wireguard.NewWireguard(wgConfig, wgc, nl)
	if err != nil {
		return nil, err
	}
	return wg, nil
}

func GetOverlayIP(ip string) string {
	tokens := strings.Split(ip, ".")
	return strings.Join([]string{NetworkPrefix, tokens[1], tokens[2], tokens[3]}, ".")
}
