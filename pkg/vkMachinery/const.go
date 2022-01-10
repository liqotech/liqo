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

package vkMachinery

import "path/filepath"

// VKCertsRootPath defines the path where VK certificates are stored.
const (
	VKCertsRootPath   = "/etc/virtual-kubelet/certs"
	VKCertsVolumeName = "virtual-kubelet-crt"
	VKClusterRoleName = "liqo-virtual-kubelet-local"
)

// KeyLocation defines the path where the VK Key file is stored.
var KeyLocation = filepath.Join(VKCertsRootPath, "server-key.pem")

// CertLocation defines the path where the VK Certificate is stored.
var CertLocation = filepath.Join(VKCertsRootPath, "server.crt")

// CsrLocation defines the path where the VK CSR is stored.
var CsrLocation = filepath.Join(VKCertsRootPath, "server.csr")

// CsrLabels defines the labels attached to the CSR resource.
var CsrLabels = map[string]string{
	"virtual-kubelet.liqo.io/csr": "true",
}

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
