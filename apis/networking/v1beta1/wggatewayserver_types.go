// Copyright 2019-2025 The Liqo Authors
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

package v1beta1

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WgGatewayServerResource the name of the wggatewayserver resources.
var WgGatewayServerResource = "wggatewayservers"

// WgGatewayServerKind specifies the kind of the wggatewayserver resources.
var WgGatewayServerKind = "WgGatewayServer"

// WgGatewayServerGroupResource specifies the group and the resource of the wggatewayserver resources.
var WgGatewayServerGroupResource = schema.GroupResource{Group: GroupVersion.Group, Resource: WgGatewayServerResource}

// WgGatewayServerGroupVersionResource specifies the group, the version and the resource of the wggatewayserver resources.
var WgGatewayServerGroupVersionResource = GroupVersion.WithResource(WgGatewayServerResource)

// ServiceTemplate defines the template of a service.
type ServiceTemplate struct {
	// Metadata of the service.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec of the service.
	Spec corev1.ServiceSpec `json:"spec,omitempty"`
}

// DeploymentTemplate defines the template of a deployment.
type DeploymentTemplate struct {
	// Metadata of the deployment.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec of the deployment.
	Spec appsv1.DeploymentSpec `json:"spec,omitempty"`
}

// ServiceMonitorTemplate defines the template of a service monitor.
type ServiceMonitorTemplate struct {
	// Metadata of the service monitor.
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec of the service monitor.
	Spec monitoringv1.ServiceMonitorSpec `json:"spec,omitempty"`
}

// Metrics defines the metrics configuration.
type Metrics struct {
	// Enabled specifies whether the metrics are enabled.
	Enabled bool `json:"enabled"`
	// Service specifies the service template for the metrics.
	Service *ServiceTemplate `json:"service,omitempty"`
	// ServiceMonitor specifies the service monitor template for the metrics.
	ServiceMonitor *ServiceMonitorTemplate `json:"serviceMonitor,omitempty"`
}

// WgGatewayServerSpec defines the desired state of WgGatewayServer.
type WgGatewayServerSpec struct {
	// Service specifies the service template for the server.
	Service ServiceTemplate `json:"service"`
	// Deployment specifies the deployment template for the server.
	Deployment DeploymentTemplate `json:"deployment"`
	// Metrics specifies the metrics configuration for the server.
	Metrics *Metrics `json:"metrics,omitempty"`
	// SecretRef specifies the reference to the secret containing the wireguard configuration.
	// Leave it empty to let the operator create a new secret.
	SecretRef corev1.LocalObjectReference `json:"secretRef,omitempty"`
}

// WgGatewayServerStatus defines the observed state of WgGatewayServer.
type WgGatewayServerStatus struct {
	// SecretRef specifies the reference to the secret.
	SecretRef *corev1.ObjectReference `json:"secretRef,omitempty"`
	// Endpoint specifies the endpoint of the server.
	Endpoint *EndpointStatus `json:"endpoint,omitempty"`
	// InternalEndpoint specifies the endpoint for the internal network.
	InternalEndpoint *InternalGatewayEndpoint `json:"internalEndpoint,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=liqo,shortName=wgs;wggs
// +kubebuilder:subresource:status

// WgGatewayServer defines a wireguard gateway server that will accept connections from remote wireguard gateway clients.
type WgGatewayServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WgGatewayServerSpec   `json:"spec,omitempty"`
	Status WgGatewayServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WgGatewayServerList contains a list of WgGatewayServer.
type WgGatewayServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WgGatewayServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WgGatewayServer{}, &WgGatewayServerList{})
}
