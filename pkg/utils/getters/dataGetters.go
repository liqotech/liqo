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

package getters

import (
	"errors"
	"fmt"
	"strconv"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconsts "github.com/liqotech/liqo/pkg/consts"
)

// NetworkConfig holds the liqo network configuration.
type NetworkConfig struct {
	PodCIDR         string
	ExternalCIDR    string
	ServiceCIDR     string
	ReservedSubnets []string
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

// RetrieveWGEPFromNodePort retrieves the WireGuard endpoint from a NodePort service.
func RetrieveWGEPFromNodePort(svc *corev1.Service, annotationKey, portName string) (endpointIP, endpointPort string, err error) {
	// Check if the node's IP where the gatewayPod is running has been set
	endpointIP, found := svc.GetAnnotations()[annotationKey]
	if !found {
		err = fmt.Errorf("the node IP where the gateway pod is running has not yet been set as an annotation for service %q", klog.KObj(svc))
		return endpointIP, endpointPort, err
	}

	// Retrieve the endpoint port
	if endpointPort, err = retrievePortFromService(svc, portName, corev1.ServiceTypeNodePort); err != nil {
		endpointIP, endpointPort = "", ""
	}

	return endpointIP, endpointPort, err
}

// RetrieveWGEPFromLoadBalancer retrieves the WireGuard endpoint from a LoadBalancer service.
func RetrieveWGEPFromLoadBalancer(svc *corev1.Service, portName string) (endpointIP, endpointPort string, err error) {
	// Retrieve the endpoint ip.
	if endpointIP, err = retrieveIPFromService(svc, corev1.ServiceTypeLoadBalancer); err != nil {
		return endpointIP, endpointPort, err
	}

	// Retrieve the endpoint port.
	if endpointPort, err = retrievePortFromService(svc, portName, corev1.ServiceTypeLoadBalancer); err != nil {
		endpointIP, endpointPort = "", ""
	}

	return endpointIP, endpointPort, err
}

// RetrieveWGEPFromService retrieves the WireGuard endpoint from a generic service.
func RetrieveWGEPFromService(svc *corev1.Service, annotationKey, portName string) (endpointIP, endpointPort string, err error) {
	switch svc.Spec.Type {
	case corev1.ServiceTypeNodePort:
		return RetrieveWGEPFromNodePort(svc, annotationKey, portName)

	case corev1.ServiceTypeLoadBalancer:
		return RetrieveWGEPFromLoadBalancer(svc, portName)

	default:
		return endpointIP, endpointPort, fmt.Errorf("service {%s/%s} is of type {%s}, only types of {%s} and {%s} are accepted",
			svc.Namespace, svc.Name, svc.Spec.Type, corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort)
	}
}

// RetrieveWGPubKeyFromSecret retrieves the WireGuard public key from a given secret if present.
func RetrieveWGPubKeyFromSecret(secret *corev1.Secret, keyName string) (pubKey wgtypes.Key, err error) {
	// Extract the public key from the secret
	pubKeyByte, found := secret.Data[keyName]
	if !found {
		err = fmt.Errorf("no data with key %s found in secret %q", keyName, klog.KObj(secret))
		return pubKey, err
	}
	pubKey, err = wgtypes.ParseKey(string(pubKeyByte))
	if err != nil {
		err = fmt.Errorf("secret %q: invalid public key: %w", klog.KObj(secret), err)
		return pubKey, err
	}

	return pubKey, nil
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
// as saved in the ipams.net.liqo.io custom resource instance.
func RetrieveNetworkConfiguration(ipamS *netv1alpha1.IpamStorage) (*NetworkConfig, error) {
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
