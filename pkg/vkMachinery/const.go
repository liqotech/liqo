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

package vkMachinery

import "github.com/liqotech/liqo/pkg/consts"

// LocalClusterRoleName represents the name of the local cluster role associated with the virtual kubelet.
const LocalClusterRoleName = "liqo-virtual-kubelet-local"

// ServiceAccountName -> the name of the service account leveraged by the virtual kubelet.
const ServiceAccountName = "virtual-kubelet"

// ContainerName -> the name of the container used to run the virtual kubelet.
const ContainerName = "virtual-kubelet"

// CRBPrefix -> the prefix used to create the virtual kubelet cluster role binding name.
const CRBPrefix = "liqo-node-"

// KubeletBaseLabels are the static labels that are set on every VirtualKubelet.
var KubeletBaseLabels = map[string]string{
	consts.OffloadingComponentKey: consts.VirtualKubeletComponentValue,
}

// ClusterRoleBindingLabels are the static labels that are set on every ClusterRoleBinding managed by Liqo.
var ClusterRoleBindingLabels = map[string]string{
	consts.K8sAppManagedByKey: consts.LiqoAppLabelValue,
}

// MetricsAddress is the default address used to expose metrics.
const MetricsAddress = ":8082"
