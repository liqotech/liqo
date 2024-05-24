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

package virtualnode

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// Options encapsulates the arguments of the virtualnode command.
type Options struct {
	createOptions *rest.CreateOptions
	deleteOptions *rest.DeleteOptions

	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity
	namespaceManager      tenantnamespace.Manager
	createNode            bool
	kubeconfigSecretName  string
	resourceSliceName     string

	cpu    string
	memory string
	pods   string

	storageClasses      []string
	ingressClasses      []string
	loadBalancerClasses []string
	labels              map[string]string
	nodeSelector        map[string]string
}

var _ rest.API = &Options{}

// VirtualNode returns the rest API for the virtualnode command.
func VirtualNode() rest.API {
	return &Options{}
}

// APIOptions returns the APIOptions for the virtualnode API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableCreate: true,
		EnableDelete: true,
	}
}
