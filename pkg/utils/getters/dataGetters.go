// Copyright 2019-2024 The Liqo Authors
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

package getters

import (
	"errors"
	"fmt"
	"strconv"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
)

// NetworkConfig holds the liqo network configuration.
type NetworkConfig struct {
	PodCIDR         string
	ExternalCIDR    string
	ServiceCIDR     string
	ReservedSubnets []string
}

// RetrieveRemoteClusterIDFromNode retrieves the remote cluster ID from a given node object.
// If the node has no RemoteCLusterID label, it returns a void string without error.
func RetrieveRemoteClusterIDFromNode(node *corev1.Node) (string, error) {
	nodeLabels := node.GetLabels()
	if nodeLabels == nil {
		return "", fmt.Errorf("node has no labels")
	}
	remoteClusterID, ok := nodeLabels[liqoconsts.RemoteClusterID]
	if !ok {
		return "", nil
	}
	return remoteClusterID, nil
}

// RetrieveClusterIDFromConfigMap retrieves ClusterIdentity from a given configmap.
func RetrieveClusterIDFromConfigMap(cm *corev1.ConfigMap) (*discoveryv1alpha1.ClusterIdentity, error) {
	id, found := cm.Data[liqoconsts.ClusterIDConfigMapKey]
	if !found {
		return nil, fmt.Errorf("unable to get cluster ID: field {%s} not found in configmap {%s/%s}",
			liqoconsts.ClusterIDConfigMapKey, cm.Namespace, cm.Name)
	}

	name, found := cm.Data[liqoconsts.ClusterNameConfigMapKey]
	if !found {
		return nil, fmt.Errorf("unable to get cluster name: field {%s} not found in configmap {%s/%s}",
			liqoconsts.ClusterNameConfigMapKey, cm.Namespace, cm.Name)
	}

	return &discoveryv1alpha1.ClusterIdentity{
		ClusterID:   id,
		ClusterName: name,
	}, nil
}

// RetrieveEndpointFromService retrieves an ip address and port from a given service object
// based on the service and port name.
func RetrieveEndpointFromService(svc *corev1.Service, svcType corev1.ServiceType, portName string) (endpointIP, endpointPort string, err error) {
	// Retrieve the endpoint ip
	if endpointIP, err = retrieveIPFromService(svc, svcType); err != nil {
		return endpointIP, endpointPort, err
	}

	// Retrieve the endpoint port
	if endpointPort, err = retrievePortFromService(svc, portName, svcType); err != nil {
		endpointIP, endpointPort = "", ""
	}

	return endpointIP, endpointPort, err
}

// retrieveIPFromService given a service and the type of the service, the function
// returns the ip address for the service based on the type. The nodePort service type
// does not have a specific ip address, so we return an error.
func retrieveIPFromService(svc *corev1.Service, serviceType corev1.ServiceType) (string, error) {
	switch serviceType {
	case corev1.ServiceTypeClusterIP:
		if svc.Spec.ClusterIP != "" {
			return svc.Spec.ClusterIP, nil
		}
		return "", fmt.Errorf("the clusterIP address for service {%s/%s} of type {%s} has not been set",
			svc.Namespace, svc.Name, svc.Spec.Type)
	case corev1.ServiceTypeLoadBalancer:
		var endpointIP string
		errorMsg := fmt.Sprintf("the ingress address for service {%s/%s} of type {%s} has not been set",
			svc.Namespace, svc.Name, svc.Spec.Type)
		// Check if the ingress IP has been set.
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return "", errors.New(errorMsg)
		}
		// Retrieve the endpoint address
		if svc.Status.LoadBalancer.Ingress[0].IP != "" {
			endpointIP = svc.Status.LoadBalancer.Ingress[0].IP
		} else if svc.Status.LoadBalancer.Ingress[0].Hostname != "" {
			endpointIP = svc.Status.LoadBalancer.Ingress[0].Hostname
		}
		if endpointIP != "" {
			return endpointIP, nil
		}
		return "", errors.New(errorMsg)
	default:
		return "", fmt.Errorf("service {%s/%s} is of type {%s}, only types of {%s} and {%s} are accepted",
			svc.Namespace, svc.Name, svc.Spec.Type, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeClusterIP)
	}
}

func retrievePortFromService(svc *corev1.Service, portName string, portType corev1.ServiceType) (string, error) {
	switch portType {
	case corev1.ServiceTypeClusterIP, corev1.ServiceTypeLoadBalancer:
		for _, port := range svc.Spec.Ports {
			if port.Name == portName {
				if port.Port == 0 {
					return "", fmt.Errorf("the clusterIP port for service {%s/%s} of type {%s} has not been set",
						svc.Namespace, svc.Name, svc.Spec.Type)
				}
				return strconv.FormatInt(int64(port.Port), 10), nil
			}
		}
	case corev1.ServiceTypeNodePort:
		for _, port := range svc.Spec.Ports {
			if port.Name == portName {
				if port.NodePort == 0 {
					return "", fmt.Errorf("the clusterIP port for service {%s/%s} of type {%s} has not been set",
						svc.Namespace, svc.Name, svc.Spec.Type)
				}
				return strconv.FormatInt(int64(port.NodePort), 10), nil
			}
		}
	default:
		return "", fmt.Errorf("service {%s/%s} is of type {%s}, only types of {%s}, {%s} and {%s} are accepted",
			svc.Namespace, svc.Name, svc.Spec.Type, corev1.ServiceTypeClusterIP, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort)
	}

	return "", fmt.Errorf("port {%s} not found in service {%s/%s} of type {%s}",
		portName, svc.Namespace, svc.Name, svc.Spec.Type)
}

// RetrieveNetworkConfiguration returns the podCIDR, serviceCIDR, reservedSubnets and the externalCIDR
// as saved in the ipamstorages.ipam.liqo.io custom resource instance.
func RetrieveNetworkConfiguration(ipamS *ipamv1alpha1.IpamStorage) (*NetworkConfig, error) {
	if ipamS.Spec.PodCIDR == "" {
		return nil, fmt.Errorf("unable to get network configuration: podCIDR is not set in resource %q", klog.KObj(ipamS))
	}

	if ipamS.Spec.ServiceCIDR == "" {
		return nil, fmt.Errorf("unable to get network configuration: serviceCIDR is not set in resource %q", klog.KObj(ipamS))
	}

	if ipamS.Spec.ExternalCIDR == "" {
		return nil, fmt.Errorf("unable to get network configuration: externalCIDR is not set %q", klog.KObj(ipamS))
	}

	return &NetworkConfig{
		PodCIDR:         ipamS.Spec.PodCIDR,
		ServiceCIDR:     ipamS.Spec.ServiceCIDR,
		ExternalCIDR:    ipamS.Spec.ExternalCIDR,
		ReservedSubnets: ipamS.Spec.ReservedSubnets,
	}, nil
}

// RetrieveClusterIDsFromVirtualNodes returns the remote cluster IDs in a list of VirtualNodes avoiding duplicates.
func RetrieveClusterIDsFromVirtualNodes(virtualNodes *virtualkubeletv1alpha1.VirtualNodeList) []string {
	clusterIDs := make(map[string]interface{})
	for i := range virtualNodes.Items {
		clusterIDs[virtualNodes.Items[i].Spec.ClusterIdentity.ClusterID] = nil
	}
	return maps.Keys(clusterIDs)
}

// RetrieveClusterIDsFromObjectsLabels returns the remote cluster IDs in a list of objects avoiding duplicates.
func RetrieveClusterIDsFromObjectsLabels[T metav1.Object](objectList []T) []string {
	clusterIDs := make(map[string]interface{})
	for i := range objectList {
		labels := objectList[i].GetLabels()
		if labels == nil {
			continue
		}
		clusterID, ok := labels[liqoconsts.RemoteClusterID]
		if !ok {
			continue
		}
		clusterIDs[clusterID] = nil
	}
	return maps.Keys(clusterIDs)
}
