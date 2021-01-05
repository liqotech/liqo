package wireguard

import (
	"fmt"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	"net"
	"reflect"
	"strconv"
	"time"
)

type WgConfig struct {
	Name      string       //device name
	IPAddress string       //ip address in CIDR notation (x.x.x.x/xx)
	Mtu       int          //mtu to be set on interface
	Port      *int         //listening port
	PriKey    *wgtypes.Key //private key
	PubKey    *wgtypes.Key //public key
}

type Wireguard struct {
	Client             // client used to interact with the wireguard implementation in use
	Netlinker          // the wireguard link living in the network namespace
	conf      WgConfig // wireguard configuration
}

func NewWireguard(config WgConfig, c Client, nl Netlinker) (*Wireguard, error) {
	var err error
	w := &Wireguard{
		Client:    c,
		Netlinker: nl,
		conf:      config,
	}
	// create and set up the interface
	if err = w.createLink(config.Name); err != nil {
		return nil, err
	}
	// if something goes wrong we make sure to close the client connection
	defer func() {
		if err != nil {
			if e := w.close(); e != nil {
				klog.Errorf("Failed to close client %v", e)
			}
			w.Client = nil
		}
	}()
	//configures the device
	peerConfigs := make([]wgtypes.PeerConfig, 0)
	cfg := wgtypes.Config{
		PrivateKey:   config.PriKey,
		ListenPort:   config.Port,
		FirewallMark: nil,
		ReplacePeers: true,
		Peers:        peerConfigs,
	}
	if err = w.configureDevice(config.Name, cfg); err != nil {
		return nil, err
	}
	if err = w.setMTU(config.Mtu); err != nil {
		return nil, err
	}
	if err = w.addIP(config.IPAddress); err != nil {
		return nil, err
	}
	return w, nil
}

// it adds a new peer with the given configuration to the wireguard device
func (w *Wireguard) AddPeer(pubkey, endpointIP, listeningPort string, allowedIPs []string, keepAlive *time.Duration) error {
	key, err := wgtypes.ParseKey(pubkey)
	if err != nil {
		return err
	}
	epIP := net.ParseIP(endpointIP)
	if epIP == nil {
		return fmt.Errorf("while parsing endpoint IP %s we got nil values, it sees to be an invalid value", endpointIP)
	}
	//convert port from string to int
	port, err := strconv.ParseInt(listeningPort, 10, 0)
	if err != nil {
		return err
	}
	var IPs = []net.IPNet{}
	for _, subnet := range allowedIPs {
		_, s, err := net.ParseCIDR(subnet)
		if err != nil {
			return err
		}
		IPs = append(IPs, *s)
	}

	//check if the peer exists
	oldPeer, err := w.getPeer(pubkey)
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if !errdefs.IsNotFound(err) {
		if epIP.String() != oldPeer.Endpoint.IP.String() || int(port) != oldPeer.Endpoint.Port || reflect.DeepEqual(IPs, oldPeer.AllowedIPs) {
			err = w.configureDevice(w.GetDeviceName(), wgtypes.Config{
				ReplacePeers: false,
				Peers: []wgtypes.PeerConfig{{PublicKey: key,
					Remove: true,
				}},
			})
			if err != nil {
				return err
			}
		}
	}

	err = w.configureDevice(w.GetDeviceName(), wgtypes.Config{
		ReplacePeers: false,
		Peers: []wgtypes.PeerConfig{{
			PublicKey:    key,
			Remove:       false,
			UpdateOnly:   false,
			PresharedKey: nil,
			Endpoint: &net.UDPAddr{
				IP:   epIP,
				Port: int(port),
			},
			PersistentKeepaliveInterval: keepAlive,
			ReplaceAllowedIPs:           true,
			AllowedIPs:                  IPs,
		}},
	})
	if err != nil {
		return err
	}
	return nil
}

// it removes a peer with a given public key from the wireguard device
func (w *Wireguard) RemovePeer(pubKey string) error {
	key, err := wgtypes.ParseKey(pubKey)
	if err != nil {
		return err
	}
	peerCfg := []wgtypes.PeerConfig{
		{
			PublicKey: key,
			Remove:    true,
		},
	}
	err = w.configureDevice(w.GetDeviceName(), wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return err
	}
	return nil
}

// returns all the peers configured for the given wireguard device
func (w *Wireguard) getPeers() ([]wgtypes.Peer, error) {
	d, err := w.device(w.GetDeviceName())
	if err != nil {
		return nil, err
	}
	return d.Peers, nil
}

// given a public key it returns the peer which has the same key
func (w *Wireguard) getPeer(pubKey string) (wgtypes.Peer, error) {
	var peer wgtypes.Peer
	peers, err := w.getPeers()
	if err != nil {
		return peer, err
	}
	for _, p := range peers {
		if p.PublicKey.String() == pubKey {
			return p, nil
		}
	}
	return peer, errdefs.NotFoundf("peer with public key '%s' not found for wireguard device '%s'", pubKey, w.GetDeviceName())
}

// get name of the wireguard device
func (w *Wireguard) GetDeviceName() string {
	return w.getLinkName()
}

// get link index of the wireguard device
func (w *Wireguard) GetLinkIndex() int {
	return w.getLinkIndex()
}

// get public key of the wireguard device
func (w *Wireguard) GetPubKey() string {
	return w.conf.PubKey.String()
}
