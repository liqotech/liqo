package overlay

import (
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"io/ioutil"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"net"
	"os"
	"strconv"
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

type Overlay interface {
	AddPeer(peer OverlayPeer) error
	RemovePeer(peer OverlayPeer) error
	AddSubnet(peerName, podIP string, podCIDR *net.IPNet) error
	RemoveSubnet(peerName, podIP string) error
	GetDeviceName() string
	GetDeviceIndex() int
	GetPubKey() string
}

type OverlayPeer struct {
	Name          string
	PubKey        string
	IpAddr        string
	ListeningPort string
	AllowedIPs    []string
}

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

//create a new routing table with the given ID and name
func CreateRoutingTable(tableID int, tableName string) error {
	klog.Infof("creating routing table with ID %d and name %s", tableID, tableName)
	//file path
	rtTableFilePath := "/etc/iproute2/rt_tables"
	data := strings.Join([]string{strconv.Itoa(tableID), tableName}, "\t")
	//open the file
	file, err := os.OpenFile(rtTableFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		klog.Errorf("an error occurred while opening file %s: %v", rtTableFilePath, err)
		return err
	}
	defer file.Close()
	//write data to file
	if _, err := file.WriteString(data); err != nil {
		klog.Errorf("an error occurred while writing to file %s: %v", rtTableFilePath, err)
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

func InsertPolicyRoutingRule(tableID int, fromSubnet string) error {
	_, subnet, err := net.ParseCIDR(fromSubnet)
	if err != nil {
		klog.Errorf("an error occurred parsing CIDR %s while inserting policy routing rule: %v", fromSubnet, err)
		return err
	}
	//get existing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return err
	}
	//check if the rule already exists
	for _, r := range rules {
		if r.Src != nil {
			if r.Src.String() == subnet.String() && r.Table == tableID {
				return nil
			}
		}
	}
	rule := netlink.NewRule()
	rule.Table = tableID
	rule.Src = subnet
	klog.Infof("inserting policy routing rule for incoming traffic from subnet %s", fromSubnet)
	if err := netlink.RuleAdd(rule); err != nil && err != unix.EEXIST {
		klog.Errorf("an error occurred while inserting policy routing rule %s: %v", rule.String(), err)
		return err
	}
	return nil
}

func RemovePolicyRoutingRule(tableID int, fromSubnet string) error {
	_, subnet, err := net.ParseCIDR(fromSubnet)
	if err != nil {
		klog.Errorf("an error occurred parsing CIDR %s while removing policy routing rule: %v", fromSubnet, err)
		return err
	}
	//get existing rules
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		klog.Errorf("an error occurred while listing the policy routing rules: %v", err)
		return err
	}
	//check if the rule already exists
	for _, r := range rules {
		if r.Src != nil {
			if r.Src.String() == subnet.String() && r.Table == tableID {
				klog.Infof("removing policy routing rule for incoming traffic from subnet %s", fromSubnet)
				if err := netlink.RuleDel(&r); err != nil && err != unix.ESRCH {
					klog.Errorf("an error occurred while removing policy routing rule %s: %v", r.String(), err)
					return err
				}
			}
		}
	}
	return nil
}
