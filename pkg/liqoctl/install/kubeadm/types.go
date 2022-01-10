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

package kubeadm

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/liqotech/liqo/pkg/liqoctl/install/provider"
)

const (
	providerPrefix             = "kubeadm"
	serviceCIDRParameterFilter = `--service-cluster-ip-range`
	podCIDRParameterFilter     = `--cluster-cidr`
	kubeSystemNamespaceName    = "kube-system"
)

var kubeControllerManagerLabels = map[string]string{"component": "kube-controller-manager", "tier": "control-plane"}

// Kubeadm contains the parameters required to install Liqo on a kubeadm cluster and a dedicated client to fetch
// those values.
type Kubeadm struct {
	provider.GenericProvider
	APIServer   string
	Config      *rest.Config
	PodCIDR     string
	ServiceCIDR string
	K8sClient   kubernetes.Interface
}
