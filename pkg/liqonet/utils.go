package liqonet

import (
	"context"
	"fmt"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/vishvananda/netlink"
	"golang.org/x/tools/go/ssa/interp/testdata/src/errors"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"net"
	"os"
	"syscall"
)

var (
	ShutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGKILL}
)

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

func GetPodNamespace() (string, error) {
	namespace, isSet := os.LookupEnv("POD_NAMESPACE")
	if !isSet {
		return "", errdefs.NotFound("the POD_NAMESPACE environment variable is not set as an environment variable")
	}
	return namespace, nil
}

func GetNodeName() (string, error) {
	nodeName, isSet := os.LookupEnv("NODE_NAME")
	if !isSet {
		return nodeName, errdefs.NotFound("NODE_NAME environment variable has not been set. check you manifest file")
	}
	return nodeName, nil
}

func GetNodePodCIDR(nodeName string, clientSet kubernetes.Interface) (string, error) {
	//get the node by name
	node, err := clientSet.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	//we do not check here if the field is set or not, it is done by the module who consumes it
	//it is an optional field
	return node.Spec.PodCIDR, nil
}

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

// Helper functions to check if a string is contained in a slice of strings.
func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// Helper functions to check and remove string from a slice of strings.
func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

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

func EnableIPForwarding() error {
	err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0600)
	if err != nil {
		return fmt.Errorf("unable to enable ip forwaring in the gateway pod: %v", err)
	}
	return nil
}

func GetDefaultIfaceName() (string, error) {
	//search for the default route and return the link associated to the route
	//we consider only the ipv4 routes
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
	//get default link
	defualtIface, err := netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		return "", err
	}
	return defualtIface.Attrs().Name, nil
}

func DeleteIFaceByIndex(ifaceIndex int) error {
	existingIface, err := netlink.LinkByIndex(ifaceIndex)
	if err != nil {
		klog.Errorf("unable to retrieve tunnel interface: %v", err)
		return err
	}
	//Remove the existing gre interface
	if err = netlink.LinkDel(existingIface); err != nil {
		klog.Errorf("unable to delete the tunnel after the tunnelEndpoint CR has been removed: %v", err)
		return err
	}
	return err
}
