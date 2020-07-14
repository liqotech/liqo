package liqonet

import (
	"bytes"
	"fmt"
	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/liqoTech/liqo/internal/errdefs"
	"golang.org/x/tools/go/ssa/interp/testdata/src/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"net"
	"os"
	"strings"
)

const (
	RouteOpLabelKey = "rouOp"
	TunOpLabelKey   = "tunOp"
)

func getPodIP() (net.IP, error) {
	ipAddress, isSet := os.LookupEnv("POD_IP")
	if !isSet {
		return nil, errdefs.NotFound("the pod IP is not set")
	}
	if ipAddress == "" {
		return nil, errors.New("pod IP is not yet set")
	}
	return net.ParseIP(ipAddress), nil
}

func GetNodeName() (string, error) {
	nodeName, isSet := os.LookupEnv("NODE_NAME")
	if !isSet {
		return nodeName, errdefs.NotFound("NODE_NAME has not been set. check you manifest file")
	}
	return nodeName, nil
}

func GetClusterPodCIDR() (string, error) {
	podCIDR, isSet := os.LookupEnv("POD_CIDR")
	if !isSet {
		return podCIDR, errdefs.NotFound("POD_CIDR has not been set. check you manifest file")
	}
	return podCIDR, nil
}

func GetClusterCIDR() (string, error) {
	clusterCIDR, isSet := os.LookupEnv("CLUSTER_CIDR")
	if !isSet {
		return clusterCIDR, errdefs.NotFound("CLUSTER_CIDR has not been set. check you manifest file")
	}
	return clusterCIDR, nil
}

func getInternalIPOfNode(node corev1.Node) (string, error) {
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

func IsGatewayNode(clientset *kubernetes.Clientset) (bool, error) {
	isGatewayNode := false
	//retrieve the node which is labeled as the gateway
	nodesList, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "liqonet.liqo.io/gateway == true"})
	if err != nil {
		logger.Error(err, "Unable to list nodes with labbel 'liqonet.liqo.io/gateway=true'")
		return isGatewayNode, fmt.Errorf(" Unable to list nodes with label 'liqonet.liqo.io/gateway=true': %v", err)
	}
	if len(nodesList.Items) != 1 {
		klog.V(4).Infof("number of gateway nodes found: %d", len(nodesList.Items))
		return isGatewayNode, errdefs.NotFound("no gateway node has been found")
	}
	//check if my ip node is the same as the internal ip of the gateway node
	podIP, err := getPodIP()
	if err != nil {
		return isGatewayNode, err
	}
	internalIP, err := getInternalIPOfNode(nodesList.Items[0])
	if err != nil {
		return isGatewayNode, fmt.Errorf("unable to get internal ip of the gateway node: %v", err)
	}
	if podIP.String() == internalIP {
		isGatewayNode = true
		return isGatewayNode, nil
	} else {
		return isGatewayNode, nil
	}
}
func GetGatewayVxlanIP(clientset *kubernetes.Clientset, vxlanConfig VxlanNetConfig) (string, error) {
	var gatewayVxlanIP string
	//retrieve the node which is labeled as the gateway
	nodesList, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "liqonet.liqo.io/gateway == true"})
	if err != nil {
		logger.Error(err, "Unable to list nodes with label 'liqonet.liqo.io/gateway=true'")
		return gatewayVxlanIP, fmt.Errorf(" Unable to list nodes with label 'liqonet.liqo.io/gateway=true': %v", err)
	}
	if len(nodesList.Items) != 1 {
		klog.V(4).Infof("number of gateway nodes found: %d", len(nodesList.Items))
		return gatewayVxlanIP, errdefs.NotFound("no gateway node has been found")
	}
	internalIP, err := getInternalIPOfNode(nodesList.Items[0])
	if err != nil {
		return gatewayVxlanIP, fmt.Errorf("unable to get internal ip of the gateway node: %v", err)
	}
	token := strings.Split(vxlanConfig.Network, "/")
	vxlanNet := token[0]
	//derive IP for the vxlan device
	//take the last octet of the podIP
	//TODO: use & and | operators with masks
	temp := strings.Split(internalIP, ".")
	temp1 := strings.Split(vxlanNet, ".")
	gatewayVxlanIP = temp1[0] + "." + temp1[1] + "." + temp1[2] + "." + temp[3]
	return gatewayVxlanIP, nil
}
func getRemoteVTEPS(clientset *kubernetes.Clientset) ([]string, error) {
	var remoteVTEP []string
	nodesList, err := clientset.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "type != virtual-node"})
	if err != nil {
		logger.Error(err, "Unable to list nodes with label 'liqonet.liqo.io/gateway=true'")
		return nil, fmt.Errorf(" Unable to list nodes with label 'type != virtual-node': %v", err)
	}
	//get my podIP so i don't put consider it a s VTEP
	podIP, err := getPodIP()
	if err != nil {
		return nil, fmt.Errorf("unable to get pod ip while getting remoteVTEPs: %v", err)
	}
	//populate the VTEPs
	for _, node := range nodesList.Items {
		internalIP, err := getInternalIPOfNode(node)
		if err != nil {
			//Log the error but don't exit
			logger.Error(err, "unable to get internal ip of the node named -> %s", node.Name)
		}
		if internalIP != podIP.String() {
			remoteVTEP = append(remoteVTEP, internalIP)
		}
	}
	return remoteVTEP, nil
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

func VerifyNoOverlap(subnets map[string]*net.IPNet, newNet *net.IPNet) bool {
	firstLastIP := make([][]net.IP, 1)

	for _, value := range subnets {
		if bytes.Compare(value.Mask, newNet.Mask) <= 0 {
			first, last := cidr.AddressRange(newNet)
			firstLastIP[0] = []net.IP{first, last}
			if value.Contains(firstLastIP[0][0]) || value.Contains(firstLastIP[0][1]) {
				klog.Infof("the subnets %s and %s overlaps", value.String(), newNet.String())
				return true
			}
		} else {
			first, last := cidr.AddressRange(value)
			firstLastIP[0] = []net.IP{first, last}
			if newNet.Contains(firstLastIP[0][0]) || newNet.Contains(firstLastIP[0][1]) {
				klog.Infof("the subnets %s and %s overlaps", value.String(), newNet.String())
				return true
			}
		}
	}
	return false
}

func SetLabelHandler(labelKey, labelValue string, mapToUpdate map[string]string) map[string]string {
	if mapToUpdate == nil {
		mapToUpdate = make(map[string]string)
	}
	mapToUpdate[labelKey] = labelValue
	return mapToUpdate
}
