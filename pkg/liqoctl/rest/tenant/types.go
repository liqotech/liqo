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

package tenant

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

// Options encapsulates the arguments of the tennant command.
type Options struct {
	generateOptions  *rest.GenerateOptions
	namespaceManager tenantnamespace.Manager

	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity
	nonce                 string
	proxyURL              string
}

var _ rest.API = &Options{}

// Tenant returns the rest API for the tenant command.
func Tenant() rest.API {
	return &Options{}
}

// APIOptions returns the APIOptions for the tenant API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableGenerate: true,
	}
}
