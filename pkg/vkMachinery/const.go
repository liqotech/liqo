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

package vkMachinery

// LocalClusterRoleName represents the name of the local cluster role associated with the virtual kubelet.
const LocalClusterRoleName = "liqo-virtual-kubelet-local"

// ServiceAccountName -> the name of the service account leveraged by the virtual kubelet.
const ServiceAccountName = "virtual-kubelet"

// CRBPrefix -> the prefix used to create the virtual kubelet cluster role binding name.
const CRBPrefix = "liqo-virtual-kubelet-"

// KubeletBaseLabels are the static labels that are set on every VirtualKubelet.
var KubeletBaseLabels = map[string]string{
	"app.kubernetes.io/name":       "virtual-kubelet",
	"app.kubernetes.io/instance":   "virtual-kubelet",
	"app.kubernetes.io/managed-by": "advertisementoperator",
	"app.kubernetes.io/component":  "virtual-kubelet",
	"app.kubernetes.io/part-of":    "liqo",
}

// ClusterRoleBindingLabels are the static labels that are set on every ClusterRoleBinding managed by the Advertisement Operator.
var ClusterRoleBindingLabels = map[string]string{
	"app.kubernetes.io/managed-by": "advertisementoperator",
}

// MetricsAddress is the default address used to expose metrics.
const MetricsAddress = ":8080"
