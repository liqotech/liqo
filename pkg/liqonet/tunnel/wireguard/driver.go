// Copyright 2019-2022 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	discv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/conncheck"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/metrics"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/resolver"
	liqonetutils "github.com/liqotech/liqo/pkg/liqonet/utils"
)

// Registering the driver as available.
func init() {
	tunnel.AddDriver(liqoconst.DriverName, NewDriver)
}

type wgConfig struct {
	// listening port.
	port int
	// private key.
	priKey wgtypes.Key
	// public key.
	pubKey wgtypes.Key
	// iFaceMTU  mtu of wg interface.
	iFaceMTU int
}

// ResolverFunc type of function that knows how to resolve an ip address belonging to
// ipv4 or ipv6 family.
type ResolverFunc func(address string) (*net.IPAddr, error)

// Wireguard a wrapper for the wireguard device and its configuration.
type Wireguard struct {
	metrics.Metrics
	// connections key is a clusterID.
	connections      map[string]*netv1alpha1.Connection
	connectionsMutex sync.RWMutex
	// connectedClusterIdentities key is the peer's public key.
	connectedClusterIdentities map[wgtypes.Key]*discv1alpha1.ClusterIdentity
	client                     *wgctrl.Client
	link                       netlink.Link
	conf                       wgConfig
	Connchecker                *conncheck.ConnChecker
}

// NewDriver creates a new WireGuard driver.
func NewDriver(k8sClient k8s.Interface, namespace string, config tunnel.Config) (tunnel.Driver, error) {
	var err error
	w := Wireguard{
		connections:                make(map[string]*netv1alpha1.Connection),
		connectedClusterIdentities: make(map[wgtypes.Key]*discv1alpha1.ClusterIdentity),
		conf: wgConfig{
			port:     config.ListeningPort,
			iFaceMTU: config.MTU,
		},
	}
	err = w.setKeys(k8sClient, namespace)
	if err != nil {
		return nil, err
	}
	if err = w.setWGLink(); err != nil {
		return nil, fmt.Errorf("failed to setup %s link: %w", liqoconst.DriverName, err)
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

	// configure the device. the device is still down.
	peerConfigs := make([]wgtypes.PeerConfig, 0)
	cfg := wgtypes.Config{
		PrivateKey:   &w.conf.priKey,
		ListenPort:   &w.conf.port,
		FirewallMark: nil,
		ReplacePeers: true,
		Peers:        peerConfigs,
	}
	if err = w.client.ConfigureDevice(liqoconst.DeviceName, cfg); err != nil {
		return nil, fmt.Errorf("failed to configure WireGuard device: %w", err)
	}

	klog.Infof("created %s interface named %s with publicKey %s", liqoconst.DriverName, liqoconst.DeviceName, w.conf.pubKey.String())

	return &w, nil
}

// Init initializes the Wireguard device.
func (w *Wireguard) Init() error {
	// ip link set $DefaultDeviceName up.
	if err := netlink.LinkSetUp(w.link); err != nil {
		return fmt.Errorf("failed to bring up WireGuard device: %w", err)
	}

	if err := netlink.LinkSetMTU(w.link, w.conf.iFaceMTU); err != nil {
		return fmt.Errorf("failed to set MTU for interface %s: %w", liqoconst.DeviceName, err)
	}

	klog.Infof("%s interface named %s, is up on i/f number %d, listening on port :%d, with key %s", liqoconst.DriverName,
		w.link.Attrs().Name, w.link.Attrs().Index, w.conf.port, w.conf.pubKey)
	return nil
}

// ConnectToEndpoint connects to a remote cluster described by the given tep.
// updateStatusCallback is a function used by conncheck to update TunnelEndpoint connected status.
func (w *Wireguard) ConnectToEndpoint(tep *netv1alpha1.TunnelEndpoint, updateStatus conncheck.UpdateFunc) (*netv1alpha1.Connection, error) {
	// parse allowed IPs.
	allowedIPs, stringAllowedIPs, err := getAllowedIPs(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// parse remote public key.
	remoteKey, err := getKey(tep)
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// parse remote endpoint.
	endpoint, err := getEndpoint(tep, func(address string) (*net.IPAddr, error) {
		return resolver.Resolve(context.TODO(), address)
	})
	if err != nil {
		return newConnectionOnError(err.Error()), err
	}

	// delete or update old peers for ClusterID.
	w.connectionsMutex.RLock()
	oldCon, found := w.connections[tep.Spec.ClusterIdentity.ClusterID]
	w.connectionsMutex.RUnlock()
	if found {
		// check if the peer configuration is updated.
		if stringAllowedIPs == oldCon.PeerConfiguration[liqoconst.WgAllowedIPs] &&
			remoteKey.String() == oldCon.PeerConfiguration[liqoconst.PublicKey] &&
			endpoint.IP.String() == oldCon.PeerConfiguration[liqoconst.WgEndpointIP] &&
			strconv.Itoa(endpoint.Port) == oldCon.PeerConfiguration[liqoconst.ListeningPort] {
			// Update connection status.
			return &tep.Status.Connection, nil
		}
		// If the configuration has changed then remove the peer.
		klog.V(4).Infof("updating peer configuration for cluster %s", tep.Spec.ClusterIdentity)
		err = w.client.ConfigureDevice(liqoconst.DeviceName, wgtypes.Config{
			ReplacePeers: false,
			Peers: []wgtypes.PeerConfig{{PublicKey: *remoteKey,
				Remove: true,
			}},
		})

		w.Connchecker.DelAndStopSender(tep.Spec.ClusterIdentity.ClusterID)

		if err != nil {
			return newConnectionOnError(err.Error()), fmt.Errorf("failed to configure peer with cluster %s: %w", tep.Spec.ClusterIdentity, err)
		}
	} else {
		klog.V(4).Infof("Connecting cluster %s endpoint %s with publicKey %s",
			tep.Spec.ClusterIdentity, endpoint.IP.String(), remoteKey)
	}

	_, externalCIDR := liqonetutils.GetExternalCIDRS(tep)
	pingIP, err := liqonetutils.GetTunnelIP(externalCIDR)

	if err != nil {
		return nil, fmt.Errorf("unable to get the tunnel ip: %w", err)
	}

	// configure peer.
	peerCfg := []wgtypes.PeerConfig{{
		PublicKey:         *remoteKey,
		Remove:            false,
		UpdateOnly:        false,
		Endpoint:          endpoint,
		ReplaceAllowedIPs: true,
		AllowedIPs:        allowedIPs,
	}}

	err = w.client.ConfigureDevice(liqoconst.DeviceName, wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return newConnectionOnError(err.Error()), fmt.Errorf("failed to configure peer with cluster %s: %w", tep.Spec.ClusterIdentity, err)
	}

	c := &netv1alpha1.Connection{
		Status:        netv1alpha1.Connecting,
		StatusMessage: netv1alpha1.ConnectingMessage,
		PeerConfiguration: map[string]string{liqoconst.ListeningPort: strconv.Itoa(endpoint.Port), liqoconst.WgEndpointIP: endpoint.IP.String(),
			liqoconst.WgAllowedIPs: stringAllowedIPs, liqoconst.PublicKey: remoteKey.String()},
		Latency: netv1alpha1.ConnectionLatency{
			Value:     liqoconst.NotApplicable,
			Timestamp: metav1.Time{Time: time.Now()},
		},
	}
	w.connectionsMutex.Lock()
	w.connections[tep.Spec.ClusterIdentity.ClusterID] = c
	w.connectionsMutex.Unlock()

	w.connectedClusterIdentities[*remoteKey] = &tep.Spec.ClusterIdentity

	klog.Infof("%s -> starting conncheck sender", tep.Spec.ClusterIdentity)

	go w.Connchecker.AddAndRunSender(tep.Spec.ClusterIdentity.ClusterID, pingIP, updateStatus)

	klog.V(4).Infof("Done connecting cluster peer %s@%s", tep.Spec.ClusterIdentity, endpoint.String())
	return c, nil
}

// DisconnectFromEndpoint disconnects a remote cluster described by the given tep.
func (w *Wireguard) DisconnectFromEndpoint(tep *netv1alpha1.TunnelEndpoint) error {
	klog.V(4).Infof("Removing connection with cluster %s", tep.Spec.ClusterIdentity)

	s, found := tep.Status.Connection.PeerConfiguration[liqoconst.PublicKey]
	if !found {
		klog.V(4).Infof("no tunnel configured for cluster %s, nothing to be removed", tep.Spec.ClusterIdentity)
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
	err = w.client.ConfigureDevice(liqoconst.DeviceName, wgtypes.Config{
		ReplacePeers: false,
		Peers:        peerCfg,
	})
	if err != nil {
		return fmt.Errorf("failed to remove WireGuard peer with cluster %s: %w", tep.Spec.ClusterIdentity, err)
	}

	remoteKey, err := getKey(tep)
	if err == nil {
		delete(w.connectedClusterIdentities, *remoteKey)
		klog.V(4).Infof("Done removing WireGuard peer with cluster %s", tep.Spec.ClusterIdentity)
	} else {
		klog.Errorf("failed to get public key for cluster %s: %v", tep.Spec.ClusterIdentity, err)
	}

	w.connectionsMutex.Lock()
	delete(w.connections, tep.Spec.ClusterIdentity.ClusterID)
	w.connectionsMutex.Unlock()

	w.Connchecker.DelAndStopSender(tep.Spec.ClusterIdentity.ClusterID)

	return nil
}

// GetLink returns the netlink.Link referred to the wireguard device.
func (w *Wireguard) GetLink() netlink.Link {
	return w.link
}

// Close remove the wireguard device from the host.
func (w *Wireguard) Close() error {
	// it removes the wireguard interface.
	var err error
	if link, err := netlink.LinkByName(liqoconst.DeviceName); err == nil {
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
func (w *Wireguard) setWGLink() error {
	var err error
	// delete existing wg device if needed.
	if link, err := netlink.LinkByName(liqoconst.DeviceName); err == nil {
		// delete existing device.
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete existing WireGuard device: %w", err)
		}
	}
	// create the wg device (ip link add dev $DefaultDeviceName type wireguard).
	la := netlink.NewLinkAttrs()
	la.Name = liqoconst.DeviceName
	la.MTU = w.conf.iFaceMTU
	link := &netlink.GenericLink{
		LinkAttrs: la,
		LinkType:  "wireguard",
	}

	if err = netlink.LinkAdd(link); err != nil && !errors.Is(err, unix.EOPNOTSUPP) {
		return fmt.Errorf("failed to add wireguard device %q: %w", liqoconst.DeviceName, err)
	}
	if errors.Is(err, unix.EOPNOTSUPP) {
		klog.Warningf("wireguard kernel module not present, falling back to the userspace implementation")

		cmd := exec.Command("/usr/bin/boringtun", liqoconst.DeviceName, "--disable-drop-privileges", "true") //nolint:gosec //we leave it as it is
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		if err != nil {
			outStr, errStr := stdout.String(), stderr.String()
			fmt.Printf("out:\n%s\nerr:\n%s\n", outStr, errStr)
			return fmt.Errorf("failed to add wireguard devices '%s': %w", liqoconst.DeviceName, err)
		}
		if w.link, err = netlink.LinkByName(liqoconst.DeviceName); err != nil {
			return fmt.Errorf("failed to get wireguard device '%s': %w", liqoconst.DeviceName, err)
		}
	}
	w.link = link
	return nil
}

// Function that receives a TunnelEndpoint resource and extracts
// wireguard allowedIPs. They are returned as []net.IPNet and
// as a string (to accommodate comparison/storing on TEP resource).
func getAllowedIPs(tep *netv1alpha1.TunnelEndpoint) ([]net.IPNet, string, error) {
	_, remotePodCIDR := liqonetutils.GetPodCIDRS(tep)
	_, remoteExternalCIDR := liqonetutils.GetExternalCIDRS(tep)

	_, podCIDR, err := net.ParseCIDR(remotePodCIDR)
	if err != nil {
		return nil, "", fmt.Errorf("unable to parse podCIDR %s for cluster %s: %w", remotePodCIDR, tep.Spec.ClusterIdentity, err)
	}
	_, externalCIDR, err := net.ParseCIDR(remoteExternalCIDR)
	if err != nil {
		return nil, "", fmt.Errorf("unable to parse externalCIDR %s for cluster %s: %w", remoteExternalCIDR, tep.Spec.ClusterIdentity, err)
	}
	return []net.IPNet{*podCIDR, *externalCIDR}, fmt.Sprintf("%s, %s", remotePodCIDR, remoteExternalCIDR), nil
}

func getKey(tep *netv1alpha1.TunnelEndpoint) (*wgtypes.Key, error) {
	s, found := tep.Spec.BackendConfig[liqoconst.PublicKey]
	if !found {
		return nil, fmt.Errorf("endpoint is missing public key")
	}

	key, err := wgtypes.ParseKey(s)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key %s: %w", s, err)
	}

	return &key, nil
}

func getEndpoint(tep *netv1alpha1.TunnelEndpoint, addrResolver ResolverFunc) (*net.UDPAddr, error) {
	// Get tunnel port.
	tunnelPort, err := getTunnelPortFromTep(tep)
	if err != nil {
		return nil, err
	}
	// Get tunnel ip.
	tunnelAddress, err := getTunnelAddressFromTep(tep, addrResolver)
	if err != nil {
		return nil, err
	}
	return &net.UDPAddr{
		IP:   tunnelAddress.IP,
		Port: tunnelPort,
	}, nil
}

func getTunnelPortFromTep(tep *netv1alpha1.TunnelEndpoint) (int, error) {
	// Get port.
	port, found := tep.Spec.BackendConfig[liqoconst.ListeningPort]
	if !found {
		return 0, fmt.Errorf("port not found in BackendConfig map using key {%s}", liqoconst.ListeningPort)
	}
	// Convert port from string to int.
	tunnelPort, err := strconv.ParseInt(port, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("unable to parse port {%s} to int: %w", port, err)
	}
	// If port is not in the correct range, then return an error.
	if tunnelPort < liqoconst.UDPMinPort || tunnelPort > liqoconst.UDPMaxPort {
		return 0, fmt.Errorf("port {%s} should be greater than {%d} and minor than {%d}", port, liqoconst.UDPMinPort, liqoconst.UDPMaxPort)
	}
	return int(tunnelPort), nil
}

func getTunnelAddressFromTep(tep *netv1alpha1.TunnelEndpoint, addrResolver ResolverFunc) (*net.IPAddr, error) {
	return addrResolver(tep.Spec.EndpointIP)
}

func newConnectionOnError(msg string) *netv1alpha1.Connection {
	return &netv1alpha1.Connection{
		Status:            netv1alpha1.ConnectionError,
		StatusMessage:     msg,
		PeerConfiguration: nil,
	}
}

func (w *Wireguard) setKeys(c k8s.Interface, namespace string) error {
	var priv, pub wgtypes.Key
	// first we check if a secret containing valid keys already exists.
	s, err := c.CoreV1().Secrets(namespace).Get(context.Background(), liqoconst.WgKeysName, metav1.GetOptions{})
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
				Name:      liqoconst.WgKeysName,
				Namespace: namespace,
				Labels:    map[string]string{liqoconst.KeysLabel: liqoconst.DriverName},
			},
			StringData: map[string]string{liqoconst.PublicKey: pub.String(), liqoconst.WgPrivateKey: priv.String()},
		}
		_, err = c.CoreV1().Secrets(namespace).Create(context.Background(), &pKey, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create the secret with name %s: %w", liqoconst.WgKeysName, err)
		}
		return nil
	}
	// get the keys from the existing secret and set them.
	privKey, found := s.Data[liqoconst.WgPrivateKey]
	if !found {
		return fmt.Errorf("no data with key '%s' found in secret %s", liqoconst.WgPrivateKey, liqoconst.WgKeysName)
	}
	priv, err = wgtypes.ParseKey(string(privKey))
	if err != nil {
		return fmt.Errorf("an error occurred while parsing the private key for the wireguard driver :%w", err)
	}
	pubKey, found := s.Data[liqoconst.PublicKey]
	if !found {
		return fmt.Errorf("no data with key '%s' found in secret %s", liqoconst.PublicKey, liqoconst.WgKeysName)
	}
	pub, err = wgtypes.ParseKey(string(pubKey))
	if err != nil {
		return fmt.Errorf("an error occurred while parsing the public key for the wireguard driver :%w", err)
	}
	w.conf.pubKey = pub
	w.conf.priKey = priv
	return nil
}

// SetNewClient set a new client used to interact with the wireguard device.
func (w *Wireguard) SetNewClient() error {
	c, err := wgctrl.New()
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("wgctrl is not available on this system")
		}
		return fmt.Errorf("failed to open wgctl client: %w", err)
	}
	w.client = c
	return nil
}

// Collect implements prometheus.Collector.
func (w *Wireguard) Collect(ch chan<- prometheus.Metric) {
	device, err := w.client.Device(liqoconst.DeviceName)
	if err != nil {
		w.MetricsErrorHandler(fmt.Errorf("error collecting wireguard metrics: %w", err), ch)
		return
	}

	for i := range device.Peers {
		publicKey := device.Peers[i].PublicKey
		labels := []string{
			liqoconst.DriverName, device.Name,
			w.connectedClusterIdentities[publicKey].ClusterID,
			w.connectedClusterIdentities[publicKey].ClusterName,
		}

		connected, err := w.Connchecker.GetConnected(w.connectedClusterIdentities[publicKey].ClusterID)
		if err != nil {
			ch <- prometheus.NewInvalidMetric(metrics.PeerIsConnected, err)
		} else {
			var result float64
			if connected {
				result = 1
			}
			ch <- prometheus.MustNewConstMetric(
				metrics.PeerIsConnected,
				prometheus.GaugeValue,
				result,
				labels...,
			)
		}

		if connected {
			ch <- prometheus.MustNewConstMetric(
				metrics.PeerReceivedBytes,
				prometheus.CounterValue,
				float64(device.Peers[i].ReceiveBytes),
				labels...,
			)

			ch <- prometheus.MustNewConstMetric(
				metrics.PeerTransmittedBytes,
				prometheus.CounterValue,
				float64(device.Peers[i].TransmitBytes),
				labels...,
			)

			latency, err := w.Connchecker.GetLatency(w.connectedClusterIdentities[publicKey].ClusterID)
			if err != nil {
				ch <- prometheus.NewInvalidMetric(metrics.PeerLatency, err)
			}
			ch <- prometheus.MustNewConstMetric(
				metrics.PeerLatency,
				prometheus.GaugeValue,
				float64(latency.Microseconds()),
				labels...,
			)
		}
	}
}
