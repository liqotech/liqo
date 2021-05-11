package wireguard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"
)

const (
	// PublicKey is the key of publicKey entry in back-end map and also for the secret containing the wireguard keys.
	PublicKey = "publicKey"
	// PrivateKey is the key of private for the secret containing the wireguard keys.
	PrivateKey = "privateKey"
	// EndpointIP is the key of the endpointIP entry in back-end map.
	EndpointIP = "endpointIP"
	// ListeningPort is the key of the listeningPort entry in the back-end map.
	ListeningPort = "port"
	// AllowedIPs is the key of the allowedIPs entry in the back-end map.
	AllowedIPs = "allowedIPs"
	// name of the network interface.
	deviceName = "liqo-wg"
	// DriverName  name of the driver which is also used as the type of the backend in tunnelendpoint CRD.
	DriverName = "wireguard"
	// name of the secret that contains the public key used by wireguard.
	keysName = "wireguard-pubkey"
	// KeysLabel label for the secret that contains the public key.
	KeysLabel   = "net.liqo.io/key"
	defaultPort = 5871
	// KeepAliveInterval interval used to send keepalive checks for the wireguard tunnels.
	KeepAliveInterval = 10 * time.Second
)

// registering the driver as available.
func init() {
	tunnel.AddDriver(DriverName, NewDriver)
}

type wgConfig struct {
	// listening port.
	port int
	// private key.
	priKey wgtypes.Key
	// public key.
	pubKey wgtypes.Key
}

type wireguard struct {
	connections map[string]*netv1alpha1.Connection
	client      *wgctrl.Client
	link        netlink.Link
	conf        wgConfig
}

// NewDriver creates a new WireGuard driver.
func NewDriver(k8sClient *k8s.Clientset, namespace string) (tunnel.Driver, error) {
	var err error
	w := wireguard{
		connections: make(map[string]*netv1alpha1.Connection),
		conf: wgConfig{
			port: defaultPort,
		},
	}
	err = w.setKeys(k8sClient, namespace)
	if err != nil {
		return nil, err
	}
	if err = w.setWGLink(); err != nil {
		return nil, fmt.Errorf("failed to setup %s link: %w", DriverName, err)
	}

	// create controller.
	if w.client, err = wgctrl.New(); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("wgctrl is not available on this system")
		}
		return nil, fmt.Errorf("failed to open wgctl client: %w", err)
	}

	defer func() {
		if err != nil {
			if e := w.client.Close(); e != nil {
				klog.Errorf("Failed to close client %w", e)
			}

			w.client = nil
		}
	}()

	port := defaultPort
	// configure the device. the device is still down.
	peerConfigs := make([]wgtypes.PeerConfig, 0)
	cfg := wgtypes.Config{
		PrivateKey:   &w.conf.priKey,
		ListenPort:   &port,
		FirewallMark: nil,
		ReplacePeers: true,
		Peers:        peerConfigs,
	}
	if err = w.client.ConfigureDevice(deviceName, cfg); err != nil {
		return nil, fmt.Errorf("failed to configure WireGuard device: %w", err)
	}
	klog.Infof("created %s interface named %s with publicKey %s", DriverName, deviceName, w.conf.pubKey.String())
	return &w, nil
}

func (w *wireguard) Init() error {
	// ip link set $DefaultDeviceName up.
	if err := netlink.LinkSetUp(w.link); err != nil {
		return fmt.Errorf("failed to bring up WireGuard device: %w", err)
	}

	if err := netlink.LinkSetMTU(w.link, 1300); err != nil {
		return fmt.Errorf("failed to set mtu for interface %s: %w", deviceName, err)
	}

	klog.Infof("%s interface named %s, is up on i/f number %d, listening on port :%d, with key %s", DriverName,
		w.link.Attrs().Name, w.link.Attrs().Index, w.conf.port, w.conf.pubKey)
	return nil
}

func (w *wireguard) ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint) (*netv1alpha1.Connection, error) {
	// parse allowed IPs.
	allowedIPs, err := getAllowedIPs(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// parse remote public key.
	remoteKey, err := getKey(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// parse remote endpoint.
	endpoint, err := getEndpoint(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// delete or update old peers for ClusterID.
	oldCon, found := w.connections[tep.Spec.ClusterID]
	if found {
		// check if the peer configuration is updated.
		if allowedIPs.String() == oldCon.PeerConfiguration[AllowedIPs] && remoteKey.String() == oldCon.PeerConfiguration[PublicKey] &&
			endpoint.IP.String() == oldCon.PeerConfiguration[EndpointIP] && strconv.Itoa(endpoint.Port) == oldCon.PeerConfiguration[ListeningPort] {
			return oldCon, nil
		}
		klog.Infof("updating peer configuration for cluster %s", tep.Spec.ClusterID)
		err = w.client.ConfigureDevice(deviceName, wgtypes.Config{
			ReplacePeers: false,
			Peers: []wgtypes.PeerConfig{{PublicKey: *remoteKey,
				Remove: true,
			}},
		})
		if err != nil {
			return newConnectionOnError(err.Error()), fmt.Errorf("failed to configure peer with clusterid %s: %w", tep.Spec.ClusterID, err)
		}
	} else {
		klog.Infof("Connecting cluster %s endpoint %s with publicKey %s",
			tep.Spec.ClusterID, endpoint.IP.String(), remoteKey)
	}

	ka := KeepAliveInterval
	// configure peer.
	peerCfg := []wgtypes.PeerConfig{{
		PublicKey:                   *remoteKey,
		Remove:                      false,
		UpdateOnly:                  false,
		Endpoint:                    endpoint,
		PersistentKeepaliveInterval: &ka,
		ReplaceAllowedIPs:           true,
		AllowedIPs:                  []net.IPNet{*allowedIPs},
	}}

	err = w.client.ConfigureDevice(deviceName, wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return newConnectionOnError(err.Error()), fmt.Errorf("failed to configure peer with clusterid %s: %w", tep.Spec.ClusterID, err)
	}
	//
	c := &netv1alpha1.Connection{
		Status:        netv1alpha1.Connected,
		StatusMessage: "Cluster peer connected",
		PeerConfiguration: map[string]string{ListeningPort: strconv.Itoa(endpoint.Port), EndpointIP: endpoint.IP.String(),
			AllowedIPs: allowedIPs.String(), PublicKey: remoteKey.String()},
	}
	w.connections[tep.Spec.ClusterID] = c
	klog.Infof("Done connecting cluster peer %s@%s", tep.Spec.ClusterID, endpoint.String())
	return c, nil
}

func (w *wireguard) DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error {
	klog.Infof("Removing connection with cluster %s", tep.Spec.ClusterID)

	s, found := tep.Status.Connection.PeerConfiguration[PublicKey]
	if !found {
		klog.Infof("no tunnel configured for cluster %s, nothing to be removed", tep.Spec.ClusterID)
		return nil
	}

	key, err := wgtypes.ParseKey(s)
	if err != nil {
		return fmt.Errorf("failed to parse public key %s: %w", s, err)
	}

	peerCfg := []wgtypes.PeerConfig{
		{
			PublicKey: key,
			Remove:    true,
		},
	}
	err = w.client.ConfigureDevice(deviceName, wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return fmt.Errorf("failed to remove WireGuard peer with clusterid %s: %w", tep.Spec.ClusterID, err)
	}

	klog.Infof("Done removing WireGuard peer with clusterid %s", tep.Spec.ClusterID)
	delete(w.connections, tep.Spec.ClusterID)

	return nil
}

func (w *wireguard) Close() error {
	// it removes the wireguard interface.
	var err error
	if link, err := netlink.LinkByName(deviceName); err == nil {
		// delete existing device
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete existing WireGuard device: %w", err)
		}
		return nil
	}
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return fmt.Errorf("failed to delete existng WireGuard device: %w", err)
}

// Create new wg link.
func (w *wireguard) setWGLink() error {
	var err error
	// delete existing wg device if needed.
	if link, err := netlink.LinkByName(deviceName); err == nil {
		// delete existing device.
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete existing WireGuard device: %w", err)
		}
	}
	// create the wg device (ip link add dev $DefaultDeviceName type wireguard).
	la := netlink.NewLinkAttrs()
	la.Name = deviceName
	la.MTU = 1300
	link := &netlink.GenericLink{
		LinkAttrs: la,
		LinkType:  "wireguard",
	}

	if err = netlink.LinkAdd(link); err != nil && !errors.Is(err, unix.EOPNOTSUPP) {
		return fmt.Errorf("failed to add wireguard device '%s': %w", deviceName, err)
	}
	if errors.Is(err, unix.EOPNOTSUPP) {
		klog.Warningf("wireguard kernel module not present, falling back to the userspace implementation")
		cmd := exec.Command("/usr/bin/boringtun", deviceName, "--disable-drop-privileges", "true")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			outStr, errStr := stdout.String(), stderr.String()
			fmt.Printf("out:\n%s\nerr:\n%s\n", outStr, errStr)
			return fmt.Errorf("failed to add wireguard devices '%s': %w", deviceName, err)
		}
		if w.link, err = netlink.LinkByName(deviceName); err != nil {
			return fmt.Errorf("failed to get wireguard device '%s': %w", deviceName, err)
		}
	}
	w.link = link
	return nil
}

func getAllowedIPs(tep *netv1alpha1.TunnelEndpoint) (*net.IPNet, error) {
	var remoteSubnet string
	// check if the remote podCIDR has been remapped.
	if tep.Status.RemoteNATPodCIDR != "None" {
		remoteSubnet = tep.Status.RemoteNATPodCIDR
	} else {
		remoteSubnet = tep.Spec.PodCIDR
	}

	_, cidr, err := net.ParseCIDR(remoteSubnet)
	if err != nil {
		return nil, fmt.Errorf("unable to parse podCIDR %s for cluster %s: %w", remoteSubnet, tep.Spec.ClusterID, err)
	}
	return cidr, nil
}

func getKey(tep *netv1alpha1.TunnelEndpoint) (*wgtypes.Key, error) {
	s, found := tep.Spec.BackendConfig[PublicKey]
	if !found {
		return nil, fmt.Errorf("endpoint is missing public key")
	}

	key, err := wgtypes.ParseKey(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key %s: %w", s, err)
	}

	return &key, nil
}

func getEndpoint(tep *netv1alpha1.TunnelEndpoint) (*net.UDPAddr, error) {
	// get port
	port, found := tep.Spec.BackendConfig[ListeningPort]
	if !found {
		return nil, fmt.Errorf("tunnelEndpoint is missing listening port")
	}
	// convert port from string to int
	listeningPort, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error while converting port %s to int: %w", port, err)
	}
	// get endpoint ip.
	remoteIP := net.ParseIP(tep.Spec.EndpointIP)
	if remoteIP == nil {
		return nil, fmt.Errorf("failed to parse remote IP %s", tep.Spec.EndpointIP)
	}
	return &net.UDPAddr{
		IP:   remoteIP,
		Port: int(listeningPort),
	}, nil
}

func newConnectionOnError(msg string) *netv1alpha1.Connection {
	return &netv1alpha1.Connection{
		Status:            netv1alpha1.ConnectionError,
		StatusMessage:     msg,
		PeerConfiguration: nil,
	}
}

func (w *wireguard) setKeys(c *k8s.Clientset, namespace string) error {
	var priv, pub wgtypes.Key
	// first we check if a secret containing valid keys already exists.
	s, err := c.CoreV1().Secrets(namespace).Get(context.Background(), keysName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	// if the secret does not exist then keys are generated and saved into a secret.
	if apierrors.IsNotFound(err) {
		// generate private and public keys
		if priv, err = wgtypes.GeneratePrivateKey(); err != nil {
			return fmt.Errorf("error generating private key for wireguard backend: %w", err)
		}
		pub = priv.PublicKey()
		w.conf.pubKey = pub
		w.conf.priKey = priv
		pKey := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      keysName,
				Namespace: namespace,
				Labels:    map[string]string{KeysLabel: DriverName},
			},
			StringData: map[string]string{PublicKey: pub.String(), PrivateKey: priv.String()},
		}
		_, err = c.CoreV1().Secrets(namespace).Create(context.Background(), &pKey, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create the secret with name %s: %w", keysName, err)
		}
		return nil
	}
	// get the keys from the existing secret and set them.
	privKey, found := s.Data[PrivateKey]
	if !found {
		return fmt.Errorf("no data with key '%s' found in secret %s", PrivateKey, keysName)
	}
	priv, err = wgtypes.ParseKey(string(privKey))
	if err != nil {
		return fmt.Errorf("an error occurred while parsing the private key for the wireguard driver :%w", err)
	}
	pubKey, found := s.Data[PublicKey]
	if !found {
		return fmt.Errorf("no data with key '%s' found in secret %s", PublicKey, keysName)
	}
	pub, err = wgtypes.ParseKey(string(pubKey))
	if err != nil {
		return fmt.Errorf("an error occurred while parsing the public key for the wireguard driver :%w", err)
	}
	w.conf.pubKey = pub
	w.conf.priKey = priv
	return nil
}
