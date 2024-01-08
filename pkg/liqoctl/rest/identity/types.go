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

package identity

import (
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqoctl/rest"
)

const (
	yamlLabel    = "yaml"
	jsonLabel    = "json"
	commandLabel = "command"
)

// Options encapsulates the arguments of the identity command.
type Options struct {
	generateOptions *rest.GenerateOptions
	updateOptions   *rest.UpdateOptions

	RemoteClusterIdentity discoveryv1alpha1.ClusterIdentity
	CertificateString     string
	PrivateKeyString      string

	remoteClusterID   string
	remoteClusterName string
}

var _ rest.API = &Options{}

// Identity returns the rest API for the identity command.
func Identity() rest.API {
	return &Options{}
}

// APIOptions returns the APIOptions for the identity API.
func (o *Options) APIOptions() *rest.APIOptions {
	return &rest.APIOptions{
		EnableGenerate: true,
		EnableUpdate:   true,
	}
}
