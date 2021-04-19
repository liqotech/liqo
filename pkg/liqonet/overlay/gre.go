package overlay

import (
	"fmt"
	"github.com/liqotech/liqo/pkg/liqonet"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"
	"math/rand"
	"net"
	"strings"
)

const (
	charSet     = "abcdefghijklmnopqrstuvwxyz"
	ifacePrefix = "liqo."
	//in recent kernel versions network interface names can be max 16 bytes long
	ttl            uint8 = 255
	ifaceSuffixLen uint8 = 8
)

type Gre struct {
	nodeName  string
	nodeIP    net.IP
	peers     map[string]*netlink.Gretun
	isGateway bool
}

func NewGretunOverlay(nodeName, nodeIP string, isGateway bool) (*Gre, error) {
	ip := net.ParseIP(nodeIP)
	if ip == nil {
		klog.Errorf("an error occurred while parsing node IP %s", nodeIP)
		return nil, fmt.Errorf("parsing ip %s returned an empty string", nodeIP)
	}
	return &Gre{
		nodeName:  nodeName,
		nodeIP:    ip,
		peers:     make(map[string]*netlink.Gretun),
		isGateway: isGateway,
	}, nil
}

func (gre *Gre) AddPeer(peer OverlayPeer) error {
	//check if the peer exists
	peerIface, ok := gre.peers[peer.Name]
	if ok {
		if !gre.compareIface(peerIface, peer) {
			if err := netlink.LinkDel(peerIface); err != nil {
				klog.Errorf("an error occurred while removing outdated peer %s: %v", peer.Name, err)
				return err
			}
			klog.Infof("existing iface %s for node %s is outdated, removing it", peerIface.Name, peer.Name)
			delete(gre.peers, peer.Name)
		} else {
			return nil
		}
	}
	ifaceName := gre.generateName(ifaceSuffixLen)
	link := &netlink.Gretun{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifaceName,
			MTU:  1300,
		},
		Local:  gre.nodeIP,
		Remote: net.ParseIP(peer.IpAddr),
		Ttl:    ttl,
	}
	if err := netlink.LinkAdd(link); err != nil {
		return err
	}
	err := netlink.LinkSetUp(link)
	if err != nil {
		klog.Errorf("unable to bring up the interface %s: %v", link.Name, err)
		return err
	}
	if gre.isGateway {
		ip := GetOverlayIP(peer.IpAddr)
		rm := liqonet.RouteManager{}
		if _, err := rm.AddRoute(ip+"/32", "", ifaceName, false); err != nil {
			klog.Errorf("an error occurred while adding route for subnet %s on interface %s: %v", ip, ifaceName, err)
			return err
		}
	} else {
		ip := GetOverlayIP(gre.nodeIP.String())
		_, subnet, err := net.ParseCIDR(ip + "/32")
		if err != nil {
			klog.Errorf("an error occurred while parsing ip %s: %v", ip, err)
			return err
		}
		if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: subnet}); err != nil {
			klog.Errorf("an error occurred while configuring ip addr %s: %v", subnet.String(), err)
			return err
		}
	}
	gre.peers[peer.Name] = link
	if err := Enable_rp_filter(ifaceName); err != nil {
		return err
	}
	klog.Infof("added GRE interface %s for node %s", ifaceName, peer.Name)
	return nil
}

func (gre *Gre) RemovePeer(peer OverlayPeer) error {
	//check if the peer exists
	peerIface, ok := gre.peers[peer.Name]
	if ok {
		klog.Infof("removing iface %s for node %s", peerIface.Name, peer.Name)
		if err := netlink.LinkDel(peerIface); err != nil {
			klog.Errorf("an error occurred while removing outdated peer %s: %v", peer.Name, err)
			return err
		}
		delete(gre.peers, peer.Name)
	}
	return nil
}

func (gre *Gre) AddSubnet(peerName, podIP string, podCIDR *net.IPNet) error {
	iface, ok := gre.peers[peerName]
	if !ok {
		klog.Infof("no peer found with name %s, unable to add route for IP %s", peerName, podIP)
		return nil
	}
	if !podCIDR.Contains(net.ParseIP(podIP)) {
		return nil
	}
	subnet := strings.Join([]string{podIP, "32"}, "/")
	rm := liqonet.RouteManager{}
	if _, err := rm.AddRoute(subnet, "", iface.Name, false); err != nil {
		klog.Errorf("an error occurred while adding route for subnet %s on interface %s: %v", subnet, iface.Name, err)
		return err
	}
	return nil
}

func (gre *Gre) RemoveSubnet(peerName, podIP string) error {
	iface, ok := gre.peers[peerName]
	if !ok {
		klog.Infof("no peer found with name %s, unable to remove route for IP %s", peerName, podIP)
		return nil
	}
	subnet := strings.Join([]string{podIP, "32"}, "/")
	_, dstNet, err := net.ParseCIDR(subnet)
	if err != nil {
		klog.Errorf("an error occurred while parsing subnet %s: %v", subnet, err)
		return err
	}
	rm := liqonet.RouteManager{}
	if err := rm.DelRoute(netlink.Route{Dst: dstNet}); err != nil {
		klog.Errorf("an error occurred while removing route for subnet %s on interface %s: %v", subnet, iface.Name, err)
		return err
	}
	return nil
}

func (gre *Gre) GetDeviceName() string {
	for _, iface := range gre.peers {
		return iface.Name
	}
	return ""
}

func (gre *Gre) GetDeviceIndex() int {
	for _, iface := range gre.peers {
		return iface.Index
	}
	return 0
}

func (gre *Gre) GetPubKey() string {
	return ""
}

func (gre *Gre) generateName(length uint8) string {
	var ifaceName string
	b := make([]byte, length)
	for {
		var conflict bool
		for i := range b {
			b[i] = charSet[rand.Intn(len(charSet))]
		}
		//build iface name
		ifaceName = strings.Join([]string{ifacePrefix, string(b)}, "")
		//check that the name is not used by the existing peers
		for _, existing := range gre.peers {
			if existing.Name == ifaceName {
				conflict = true
			}
		}
		if !conflict {
			return ifaceName
		}
	}
}

func (gre *Gre) compareIface(existing *netlink.Gretun, peer OverlayPeer) bool {
	if existing.Local.String() != gre.nodeIP.String() {
		return false
	}
	if existing.Remote.String() != peer.IpAddr {
		return false
	}
	if existing.Ttl != ttl {
		return false
	}
	return true
}
