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

package kubeadm

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
	"github.com/liqotech/liqo/pkg/liqoctl/utils"
)

var _ install.Provider = (*Options)(nil)

var kubeControllerManagerLabels = map[string]string{"component": "kube-controller-manager", "tier": "control-plane"}

// Options encapsulates the arguments of the install kubeadm command.
type Options struct {
	*install.Options
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "kubeadm" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string {
	return `Examples:
  $ {{ .Executable }} install kubeadm --cluster-labels region=europe,environment=staging \
      --reserved-subnets 172.16.0.0/16,192.16.254.0/24
`
}

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(_ *cobra.Command) {}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(ctx context.Context) error {
	const (
		serviceCIDRParameterFilter = `--service-cluster-ip-range`
		podCIDRParameterFilter     = `--cluster-cidr`
		kubeSystemNamespaceName    = "kube-system"

		defaultPodCIDR     = "172.16.0.0/16"
		defaultServiceCIDR = "10.96.0.0/12"
	)

	// Retrieve the Pod CIDR and Service CIDR based on the controller-manager configuration
	cm, err := o.KubeClient.CoreV1().Pods(kubeSystemNamespaceName).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(kubeControllerManagerLabels).AsSelector().String(),
	})
	if err != nil {
		return err
	}
	if len(cm.Items) < 1 {
		return fmt.Errorf("kube-controller-manager not found")
	}
	if len(cm.Items[0].Spec.Containers) != 1 {
		return fmt.Errorf("unexpected amount of containers in kube-controller-manager")
	}

	command := cm.Items[0].Spec.Containers[0].Command
	o.PodCIDR = utils.ExtractValuesFromArgumentListOrDefault(podCIDRParameterFilter, command, defaultPodCIDR)
	o.ServiceCIDR = utils.ExtractValuesFromArgumentListOrDefault(serviceCIDRParameterFilter, command, defaultServiceCIDR)

	return nil
}

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{}
}
