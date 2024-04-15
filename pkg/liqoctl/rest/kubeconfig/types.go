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

package kubeconfig

import (
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// Options encapsulates the arguments of the kubeconfig command.
type Options struct {
	getOptions *rest.GetOptions

	namespaceManager tenantnamespace.Manager
	identityName     string
	remoteClusterID  string
}

var _ rest.API = &Options{}

// Kubeconfig returns the rest API for the kubeconfig command.
func Kubeconfig() rest.API {
	return &Options{}
}

// APIOptions returns the APIOptions for the kubeconfig API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableGet: true,
	}
}
