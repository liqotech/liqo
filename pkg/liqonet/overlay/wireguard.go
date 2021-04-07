package overlay

import (
	"fmt"
	"github.com/liqotech/liqo/pkg/liqonet/wireguard"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"strings"
	"time"
)

var (
	wgPort    = 51871
	wgMtu     = 1300
	keepalive = 10 * time.Second
)

type Wireguard struct {
	nodeName  string
	namespace string
	nodeIP    string
	wg        *wireguard.Wireguard
	peers     map[string]string
}

func NewWireguardOverlay(nodeName, namespace, nodeIP string, k8sClient kubernetes.Interface, wgClient wireguard.Client, nlClient wireguard.Netlinker) (*Wireguard, error) {
	if k8sClient == nil {
		return nil, fmt.Errorf("k8s client should be a valid one")
	}
	if wgClient == nil {
		return nil, fmt.Errorf("wireguard client should be a valid one")
	}
	if nlClient == nil {
		return nil, fmt.Errorf("netlink client should be a valid one")
	}
	overlayIP := strings.Join([]string{GetOverlayIP(nodeIP), "4"}, "/")
	secretName := strings.Join([]string{secretPrefix, nodeName}, "")
	priv, pub, err := wireguard.GetKeys(secretName, namespace, k8sClient)
	if err != nil {
		return nil, err
	}
	wgConfig := wireguard.WgConfig{
		Name:      WgInterfacename,
		IPAddress: overlayIP,
		Mtu:       wgMtu,
		Port:      &wgPort,
		PriKey:    &priv,
		PubKey:    &pub,
	}
	wg, err := wireguard.NewWireguard(wgConfig, wgClient, nlClient)
	if err != nil {
		return nil, err
	}

	return &Wireguard{
		nodeName:  nodeName,
		namespace: namespace,
		nodeIP:    nodeIP,
		wg:        wg,
		peers:     make(map[string]string),
	}, nil
}

func (ov *Wireguard) AddPeer(peer OverlayPeer) error {
	if err := ov.wg.AddPeer(peer.PubKey, peer.IpAddr, peer.ListeningPort, peer.AllowedIPs, &keepalive); err != nil {
		return err
	}
	ov.peers[peer.Name] = peer.PubKey
	return nil
}

func (ov *Wireguard) RemovePeer(peer OverlayPeer) error {
	if err := ov.wg.RemovePeer(peer.PubKey); err != nil {
		return err
	}
	delete(ov.peers, peer.Name)
	return nil
}

func (ov *Wireguard) AddSubnet(peerName, podIP string) error {
	//check if there is a configured peer for the given nodeName
	pubKey, ok := ov.peers[peerName]
	if !ok {
		klog.Infof("no peer found with name %s, unable to add route for IP %s", peerName, podIP)
		return nil
	}
	allowedIPs := strings.Join([]string{podIP, "32"}, "/")
	err := ov.wg.AddAllowedIPs(pubKey, allowedIPs)
	if err != nil {
		klog.Errorf("an error occurred while adding subnet %s to the allowedIPs for peer %s: %v", allowedIPs, peerName, err)
		return err
	}
	return nil
}

func (ov *Wireguard) RemoveSubnet(peerName, podIP string) error {
	//check if there is a configured peer for the given nodeName
	pubKey, ok := ov.peers[peerName]
	if !ok {
		klog.Infof("no peer found with name %s, unable to remove route for IP %s", peerName, podIP)
		return nil
	}
	allowedIPs := strings.Join([]string{podIP, "32"}, "/")
	err := ov.wg.RemoveAllowedIPs(pubKey, allowedIPs)
	if err != nil {
		klog.Errorf("an error occurred while removing subnet %s to the allowedIPs for peer %s: %v", allowedIPs, peerName, err)
		return err
	}
	return nil
}

func (ov *Wireguard) GetDeviceName() string {
	return ov.wg.GetDeviceName()
}

func (ov *Wireguard) GetDeviceIndex() int {
	return ov.wg.GetLinkIndex()
}

func (ov *Wireguard) GetPubKey() string {
	return ov.wg.GetPubKey()
}
