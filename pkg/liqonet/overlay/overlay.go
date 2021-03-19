package overlay

import (
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"io/ioutil"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"net"
	"strings"
)

const (
	secretPrefix          = "overlaykeys-"
	WgInterfacename       = "liqo.overlay"
	NetworkPrefix         = "240"
	WgListeningPort       = "51871"
	PubKeyAnnotation      = "net.liqo.io/overlay.pubkey"
	NodeCIDRKeyAnnotation = "net.liqo.io/node.cidr"
	RoutingTableID        = 18952
	RoutingTableName      = "liqo"
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

//this function enables the rp_filter for the overlay interface on the gateway node
func Enable_rp_filter(ifaceName string) error {
	klog.Infof("enabling reverse path filter for interface %s", ifaceName)
	rpFilterFilePath := strings.Join([]string{"/proc/sys/net/ipv4/conf/", ifaceName, "/rp_filter"}, "")
	// Enable loose mode reverse path filtering on the overlay interface.
	err := ioutil.WriteFile(rpFilterFilePath, []byte("2"), 0600)
	if err != nil {
		klog.Errorf("an error occurred while writing to file %s: %v", rpFilterFilePath, err)
		return err
	}
	return nil
}

func SetUpDefaultRoute(tableID int, ifaceIndex int) error {
	klog.Infof("setting default route for routing table with ID %d", tableID)
	dst := &net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}
	route := netlink.Route{
		LinkIndex: ifaceIndex,
		Dst:       dst,
		Table:     tableID,
	}
	if err := netlink.RouteAdd(&route); err != nil && err != unix.EEXIST {
		klog.Errorf("an error occurred while inserting default route in table with ID %d for the interface with index %d: %v", tableID, ifaceIndex, err)
		return err
	}
	return nil
}
