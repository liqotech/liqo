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

package generic

import (
	"context"

	"github.com/spf13/cobra"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/liqotech/liqo/pkg/liqoctl/install"
)

var _ install.Provider = (*Options)(nil)

// Options encapsulates the arguments of the install command.
type Options struct {
	*install.Options
}

// New initializes a new Provider object.
func New(o *install.Options) install.Provider {
	return &Options{Options: o}
}

// Name returns the name of the provider.
func (o *Options) Name() string { return "" }

// Examples returns the examples string for the given provider.
func (o *Options) Examples() string { return "" }

// RegisterFlags registers the flags for the given provider.
func (o *Options) RegisterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.APIServer, "api-server-url", "", "The Kubernetes API Server URL (defaults to the one specified in the kubeconfig)")
	cmd.Flags().StringVar(&o.PodCIDR, "pod-cidr", "", "The Pod CIDR of the cluster")
	cmd.Flags().StringVar(&o.ServiceCIDR, "service-cidr", "", "The Service CIDR of the cluster")

	utilruntime.Must(cmd.MarkFlagRequired("pod-cidr"))
	utilruntime.Must(cmd.MarkFlagRequired("service-cidr"))
}

// Initialize performs the initialization tasks to retrieve the provider-specific parameters.
func (o *Options) Initialize(_ context.Context) error { return nil }

// Values returns the customized provider-specifc values file parameters.
func (o *Options) Values() map[string]interface{} {
	return map[string]interface{}{}
}
