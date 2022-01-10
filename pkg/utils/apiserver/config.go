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

package apiserver

import (
	"encoding/base64"
	"flag"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// config is the instance of the configuration automatically populated based on the command line parameters.
var config Config

// Config defines the configuration parameters to contact the Kubernetes API server.
type Config struct {
	// The address of the Kubernetes API Server, advertised to the peering clusters.
	Address string

	// Whether the Kubernetes API Server is exposed through a trusted certification authority,
	// which does not need to be explicitly advertised.
	TrustedCA bool

	// The certification authority trusted by the API server.
	CA string
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

// Complete completes the retrieval of the configuration, defaulting the fields if not set.
func (c *Config) Complete(restcfg *rest.Config, client kubernetes.Interface) (err error) {
	if c.Address, err = GetURL(c.Address, client); err != nil {
		return err
	}

	if !c.TrustedCA {
		if c.CA, err = retrieveAPIServerCA(restcfg); err != nil {
			return err
		}
	}

	return nil
}

// getAPIServerCA retrieves the APIServerCA, either from the CAData in the restConfig, or reading from the CAFile.
func retrieveAPIServerCA(restcfg *rest.Config) (string, error) {
	if restcfg.CAData != nil && len(restcfg.CAData) > 0 {
		// CAData available in the restConfig, encode and return it.
		return base64.StdEncoding.EncodeToString(restcfg.CAData), nil
	}
	if restcfg.CAFile != "" {
		// CAData is not available, read it from the CAFile.
		data, err := os.ReadFile(restcfg.CAFile)
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString(data), nil
	}
	return "", nil
}
