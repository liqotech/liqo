// Copyright 2019-2021 The Liqo Authors
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

package apiserver

import "flag"

// config is the instance of the configuration automatically populated based on the command line parameters.
var config Config

// Config defines the configuration parameters to contact the Kubernetes API server.
type Config struct {
	// The address of the Kubernetes API Server, advertised to the peering clusters.
	// Overrides the IP address of a control plane node, with the default port (6443).
	Address string

	// Whether the Kubernetes API Server is exposed through a trusted certification authority,
	// which does not need to be explicitly advertised.
	TrustedCA bool
}

// InitFlags initializes the flags to configure the API server parameters.
func InitFlags(flagset *flag.FlagSet) {
	if flagset == nil {
		flagset = flag.CommandLine
	}

	flagset.StringVar(&config.Address, "advertise-api-server-address", "",
		"The address of the Kubernetes API Server, advertised to the peering clusters. "+
			"If not set, it is automatically set to the IP address of a control plane node, with the default port (6443)")
	flagset.BoolVar(&config.TrustedCA, "advertise-api-server-trusted-ca", false,
		"Whether the Kubernetes API Server is exposed through a trusted certification authority, which does not need to be explicitly advertised.")
}

// GetConfig returns the API server configuration populated based on the command line parameters.
func GetConfig() Config { return config }
