package utils

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"syscall"

	"github.com/vishvananda/netlink"
	"golang.org/x/tools/go/ssa/interp/testdata/src/errors"
	"inet.af/netaddr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/liqotech/liqo/pkg/consts"
	liqoneterrors "github.com/liqotech/liqo/pkg/liqonet/errors"
)

var (
	// ShutdownSignals signals used to terminate the programs.
	ShutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGKILL}
)

// MapIPToNetwork creates a new IP address obtained by means of the old IP address and the new network.
func MapIPToNetwork(newNetwork, oldIP string) (newIP string, err error) {
	if newNetwork == consts.DefaultCIDRValue {
		return oldIP, nil
	}
	// Parse newNetwork
	ip, network, err := net.ParseCIDR(newNetwork)
	if err != nil {
		return "", err
	}
	// Get mask
	mask := network.Mask
	// Get slice of bytes for newNetwork
	// Type net.IP has underlying type []byte
	parsedNewIP := ip.To4()
	// Get oldIP as slice of bytes
	parsedOldIP := net.ParseIP(oldIP)
	if parsedOldIP == nil {
		return "", fmt.Errorf("cannot parse oldIP")
	}
	parsedOldIP = parsedOldIP.To4()
	// Substitute the last 32-mask bits of newNetwork with bits taken by the old ip
	for i := 0; i < len(mask); i++ {
		// Step 1: NOT(mask[i]) = mask[i] ^ 0xff. They are the 'host' bits
		// Step 2: BITWISE AND between the host bits and parsedOldIP[i] zeroes the network bits in parsedOldIP[i]
		// Step 3: BITWISE OR copies the result of step 2 in newIP[i]
		parsedNewIP[i] |= (mask[i] ^ 0xff) & parsedOldIP[i]
	}
	newIP = parsedNewIP.String()
	return
}

func GetPodIP() (net.IP, error) {
	ipAddress, isSet := os.LookupEnv("POD_IP")
	if !isSet {
		return nil, errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return nil, errors.New("pod IP is not yet set")
	}
	return net.ParseIP(ipAddress), nil
}

// GetPodNamespace gets the namespace of the pod passed as an environment variable.
func GetPodNamespace() (string, error) {
	namespace, isSet := os.LookupEnv("POD_NAMESPACE")
	if !isSet {
		return "", errdefs.NotFound("the POD_NAMESPACE environment variable is not set as an environment variable")
	}
	return namespace, nil
}

// GetNodeName gets the name of the node where the pod is running passed as an environment variable.
func GetNodeName() (string, error) {
	nodeName, isSet := os.LookupEnv("NODE_NAME")
	if !isSet {
		return nodeName, errdefs.NotFound("NODE_NAME environment variable has not been set. check you manifest file")
	}
	return nodeName, nil
}

// GetNodePodCIDR gets the subnet assigned to the node as podCIDR.
func GetNodePodCIDR(nodeName string, clientSet kubernetes.Interface) (string, error) {
	// get the node by name
	node, err := clientSet.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	// we do not check here if the field is set or not, it is done by the module who consumes it
	// it is an optional field
	return node.Spec.PodCIDR, nil
}

// GetInternalIPOfNode returns the first internal ip of the node if any is set.
func GetInternalIPOfNode(node *corev1.Node) (string, error) {
	var internalIp string
	for _, address := range node.Status.Addresses {
		if address.Type == "InternalIP" {
			internalIp = address.Address
			break
		}
	}
	if internalIp == "" {
		klog.V(4).Infof("internalIP of the node not found, probably is not set")
		return internalIp, errdefs.NotFound("internalIP of the node is not set")
	}
	return internalIp, nil
}

// GetClusterID returns the the clusterID et in the given config map.
func GetClusterID(client kubernetes.Interface, cmName, namespace string, backoff wait.Backoff) (string, error) {
	cmClient := client.CoreV1().ConfigMaps(namespace)
	var cm *corev1.ConfigMap
	var err error

	notFound := func(error) bool {
		klog.V(4).Info("Error while getting ClusterID ConfigMap. Retrying...")
		return k8serrors.IsNotFound(err)
	}

	klog.Info("Getting ClusterID from ConfigMap...")
	retryErr := retry.OnError(backoff, notFound, func() error {
		cm, err = cmClient.Get(context.TODO(), cmName, metav1.GetOptions{})
		return err
	})
	if retryErr != nil {
		return "", retryErr
	}

	clusterID := cm.Data[cmName]
	klog.Infof("ClusterID is '%s'", clusterID)
	return clusterID, nil
}

// EnableIPForwarding enables ipv4 forwarding on the node/pod where it is called.
func EnableIPForwarding() error {
	err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0600)
	if err != nil {
		return fmt.Errorf("unable to enable ip forwaring in the gateway pod: %v", err)
	}
	return nil
}

// GetMask retrieves the mask from a CIDR.
func GetMask(network string) uint8 {
	_, net, _ := net.ParseCIDR(network)
	ones, _ := net.Mask.Size()
	return uint8(ones)
}

// SetMask forges a new cidr from a network cidr and a mask.
func SetMask(network string, mask uint8) (string, error) {
	_, n, err := net.ParseCIDR(network)
	if err != nil {
		return "", err
	}
	newMask := net.CIDRMask(int(mask), 32)
	n.Mask = newMask
	return n.String(), nil
}

func Next(network string) (string, error) {
	prefix, err := netaddr.ParseIPPrefix(network)
	if err != nil {
		return "", err
	}
	// Step 1: Get last IP address of network
	// Step 2: Get next IP address
	firstIP := prefix.Range().To.Next()
	prefix.IP = firstIP
	return prefix.String(), nil
}

// GetPodCIDRS for a given tep the function retrieves the values for localPodCIDR and remotePodCIDR.
// Their values depend if the NAT is required or not.
func GetPodCIDRS(tep *netv1alpha1.TunnelEndpoint) (localRemappedPodCIDR, remotePodCIDR string) {
	if tep.Status.RemoteNATPodCIDR != consts.DefaultCIDRValue {
		remotePodCIDR = tep.Status.RemoteNATPodCIDR
	} else {
		remotePodCIDR = tep.Spec.PodCIDR
	}
	localRemappedPodCIDR = tep.Status.LocalNATPodCIDR
	return localRemappedPodCIDR, remotePodCIDR
}

// GetExternalCIDRS for a given tep the function retrieves the values for localExternalCIDR and remoteExternalCIDR.
// Their values depend if the NAT is required or not.
func GetExternalCIDRS(tep *netv1alpha1.TunnelEndpoint) (localExternalCIDR, remoteExternalCIDR string) {
	if tep.Status.LocalNATExternalCIDR != consts.DefaultCIDRValue {
		localExternalCIDR = tep.Status.LocalNATExternalCIDR
	} else {
		localExternalCIDR = tep.Status.LocalExternalCIDR
	}
	if tep.Status.RemoteNATExternalCIDR != consts.DefaultCIDRValue {
		remoteExternalCIDR = tep.Status.RemoteNATExternalCIDR
	} else {
		remoteExternalCIDR = tep.Spec.ExternalCIDR
	}
	return
}

// GetDefaultIfaceName returns the name of the interfaces that has the default route configured.
func GetDefaultIfaceName() (string, error) {
	// search for the default route and return the link associated to the route
	// we consider only the ipv4 routes
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}
	var route netlink.Route
	for _, route = range routes {
		if route.Dst == nil {
			break
		}
	}
	// get default link
	defualtIface, err := netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		return "", err
	}
	return defualtIface.Attrs().Name, nil
}

// DeleteIFaceByIndex deletes the interface that has the given index.
func DeleteIFaceByIndex(ifaceIndex int) error {
	existingIface, err := netlink.LinkByIndex(ifaceIndex)
	if err != nil {
		klog.Errorf("unable to retrieve tunnel interface: %v", err)
		return err
	}
	// Remove the existing gre interface
	if err = netlink.LinkDel(existingIface); err != nil {
		klog.Errorf("unable to delete the tunnel after the tunnelEndpoint CR has been removed: %v", err)
		return err
	}
	return err
}

// IsValidCIDR returns an error if the received CIDR is invalid.
func IsValidCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return err
}

// GetFirstIP returns the first IP address of a network.
func GetFirstIP(network string) (string, error) {
	firstIP, _, err := net.ParseCIDR(network)
	if err != nil {
		return "", err
	}
	return firstIP.String(), nil
}

// CheckTep checks validity of TunnelEndpoint resource fields.
func CheckTep(tep *netv1alpha1.TunnelEndpoint) error {
	if tep.Spec.ClusterID == "" {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.ClusterIDLabelName,
			Reason:    liqoneterrors.StringNotEmpty,
		}
	}
	if err := IsValidCIDR(tep.Spec.PodCIDR); err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.PodCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Spec.ExternalCIDR); err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.ExternalCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Status.LocalPodCIDR); err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.LocalPodCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Status.LocalExternalCIDR); err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.LocalExternalCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Status.LocalNATPodCIDR); tep.Status.LocalNATPodCIDR != consts.DefaultCIDRValue &&
		err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.LocalNATPodCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Status.LocalNATExternalCIDR); tep.Status.LocalNATExternalCIDR != consts.DefaultCIDRValue &&
		err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.LocalNATExternalCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Status.RemoteNATPodCIDR); tep.Status.RemoteNATPodCIDR != consts.DefaultCIDRValue &&
		err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.RemoteNATPodCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}
	if err := IsValidCIDR(tep.Status.RemoteNATExternalCIDR); tep.Status.RemoteNATExternalCIDR != consts.DefaultCIDRValue &&
		err != nil {
		return &liqoneterrors.WrongParameter{
			Parameter: consts.RemoteNATExternalCIDR,
			Reason:    liqoneterrors.ValidCIDR,
		}
	}

	return nil
}

// GetOverlayIP given an IP address it is mapped in to the overlay network,
// described by consts.OverlayNetworkPrefix. It uses the overlay prefix and the
// last three octets of the original IP address.
func GetOverlayIP(ip string) string {
	addr := net.ParseIP(ip)
	// If the ip is malformed we prevent a panic, the subsequent calls
	// that use the returned value will return an error.
	if addr == nil {
		return ""
	}
	tokens := strings.Split(ip, ".")
	return strings.Join([]string{consts.OverlayNetworkPrefix, tokens[1], tokens[2], tokens[3]}, ".")
}
