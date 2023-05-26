// Copyright 2019-2023 The Liqo Authors
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

	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
)

// VirtualKubeletOpts defines the custom options associated with the virtual kubelet deployment forging.
type VirtualKubeletOpts struct {
	// ContainerImage contains the virtual kubelet image name and tag.
	ContainerImage       string
	ExtraAnnotations     map[string]string
	ExtraLabels          map[string]string
	ExtraArgs            []string
	NodeName             string
	NodeExtraAnnotations argsutils.StringMap
	NodeExtraLabels      argsutils.StringMap
	RequestsCPU          resource.Quantity
	LimitsCPU            resource.Quantity
	RequestsRAM          resource.Quantity
	LimitsRAM            resource.Quantity
	IpamEndpoint         string
	MetricsEnabled       bool
	MetricsAddress       string
	StorageClasses       []sharingv1alpha1.StorageType
}

// VirtualKubeletOptsFlag defines the custom options flags associated with the virtual kubelet deployment forging.
type VirtualKubeletOptsFlag string

const (
	// ForeignClusterID is the flag used to specify the foreign cluster ID.
	ForeignClusterID VirtualKubeletOptsFlag = "--foreign-cluster-id"
	// ForeignClusterName is the flag used to specify the foreign cluster name.
	ForeignClusterName VirtualKubeletOptsFlag = "--foreign-cluster-name"
	// ForeignClusterKubeconfigSecretName is the flag used to specify the foreign cluster kubeconfig secret name.
	ForeignClusterKubeconfigSecretName VirtualKubeletOptsFlag = "--foreign-kubeconfig-secret-name"
	// NodeName is the flag used to specify the node name.
	NodeName VirtualKubeletOptsFlag = "--nodename"
	// NodeIP is the flag used to specify the node IP.
	NodeIP VirtualKubeletOptsFlag = "--node-ip"
	// TenantNamespace is the flag used to specify the tenant namespace.
	TenantNamespace VirtualKubeletOptsFlag = "--tenant-namespace"
	// HomeClusterID is the flag used to specify the home cluster ID.
	HomeClusterID VirtualKubeletOptsFlag = "--home-cluster-id"
	// HomeClusterName is the flag used to specify the home cluster name.
	HomeClusterName VirtualKubeletOptsFlag = "--home-cluster-name"
	// IpamEndpoint is the flag used to specify the IPAM endpoint.
	IpamEndpoint VirtualKubeletOptsFlag = "--ipam-server"
	// EnableStorage is the flag used to enable the storage.
	EnableStorage VirtualKubeletOptsFlag = "--enable-storage"
	// RemoteRealStorageClassName is the flag used to specify the remote real storage class name.
	RemoteRealStorageClassName VirtualKubeletOptsFlag = "--remote-real-storage-class-name"
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
)
