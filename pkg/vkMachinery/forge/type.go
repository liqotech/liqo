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

package forge

import (
	"k8s.io/apimachinery/pkg/api/resource"

	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// VirtualKubeletOpts defines the custom options associated with the virtual kubelet deployment forging.
type VirtualKubeletOpts struct {
	// ContainerImage contains the virtual kubelet image name and tag.
	ContainerImage       string
	ExtraAnnotations     map[string]string
	ExtraLabels          map[string]string
	ExtraArgs            []string
	NodeExtraAnnotations argsutils.StringMap
	NodeExtraLabels      argsutils.StringMap
	RequestsCPU          resource.Quantity
	LimitsCPU            resource.Quantity
	RequestsRAM          resource.Quantity
	LimitsRAM            resource.Quantity
	IpamEndpoint         string
	MetricsEnabled       bool
	MetricsAddress       string
	ReflectorsWorkers    map[string]*uint
	ReflectorsType       map[string]*string
	LocalPodCIDR         string
	LiqoNamespace        string
}

// VirtualKubeletOptsFlag defines the custom options flags associated with the virtual kubelet deployment forging.
type VirtualKubeletOptsFlag string

const (
	// ForeignClusterID is the flag used to specify the foreign cluster ID.
	ForeignClusterID VirtualKubeletOptsFlag = "--foreign-cluster-id"
	//nolint:gosec // we are not using this flag to store sensitive data
	// ForeignClusterKubeconfigSecretName is the flag used to specify the foreign cluster kubeconfig secret name.
	ForeignClusterKubeconfigSecretName VirtualKubeletOptsFlag = "--foreign-kubeconfig-secret-name"
	// NodeName is the flag used to specify the node name.
	NodeName VirtualKubeletOptsFlag = "--nodename"
	// NodeIP is the flag used to specify the node IP.
	NodeIP VirtualKubeletOptsFlag = "--node-ip"
	// TenantNamespace is the flag used to specify the tenant namespace.
	TenantNamespace VirtualKubeletOptsFlag = "--tenant-namespace"
	// LiqoNamespace is the flag used to specify the Liqo namespace.
	LiqoNamespace VirtualKubeletOptsFlag = "--liqo-namespace"
	// HomeClusterID is the flag used to specify the home cluster ID.
	HomeClusterID VirtualKubeletOptsFlag = "--home-cluster-id"
	// LocalPodCIDR is the flag used to specify the local pod CIDR.
	LocalPodCIDR VirtualKubeletOptsFlag = "--local-podcidr"
	// IpamEndpoint is the flag used to specify the IPAM endpoint.
	IpamEndpoint VirtualKubeletOptsFlag = "--ipam-server"
	// EnableStorage is the flag used to enable the storage.
	EnableStorage VirtualKubeletOptsFlag = "--enable-storage"
	// RemoteRealStorageClassName is the flag used to specify the remote real storage class name.
	RemoteRealStorageClassName VirtualKubeletOptsFlag = "--remote-real-storage-class-name"
	// EnableIngress is the flag used to enable the ingress.
	EnableIngress VirtualKubeletOptsFlag = "--enable-ingress"
	// RemoteRealIngressClassName is the flag used to specify the remote real ingress class name.
	RemoteRealIngressClassName VirtualKubeletOptsFlag = "--remote-real-ingress-class-name"
	// EnableLoadBalancer is the flag used to enable the load balancer.
	EnableLoadBalancer VirtualKubeletOptsFlag = "--enable-load-balancer"
	// RemoteRealLoadBalancerClassName is the flag used to specify the remote real load balancer class name.
	RemoteRealLoadBalancerClassName VirtualKubeletOptsFlag = "--remote-real-load-balancer-class-name"
	// NodeExtraAnnotations is the flag used to specify the node extra annotations.
	NodeExtraAnnotations VirtualKubeletOptsFlag = "--node-extra-annotations"
	// NodeExtraLabels is the flag used to specify the node extra labels.
	NodeExtraLabels VirtualKubeletOptsFlag = "--node-extra-labels"
	// MetricsEnabled is the flag used to enable the metrics.
	MetricsEnabled VirtualKubeletOptsFlag = "--metrics-enabled"
	// MetricsAddress is the flag used to specify the metrics address.
	MetricsAddress VirtualKubeletOptsFlag = "--metrics-address"
	// CreateNode is the flag used to specify if the node must be created.
	CreateNode VirtualKubeletOptsFlag = "--create-node"
	// NodeCheckNetwork is the flag used to specify if the network must be checked.
	NodeCheckNetwork VirtualKubeletOptsFlag = "--node-check-network"
)
