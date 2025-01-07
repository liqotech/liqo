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

package apiserver

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func InitFlags(flagset *pflag.FlagSet) {
	if flagset == nil {
		flagset = pflag.CommandLine
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
func (c *Config) Complete(restcfg *rest.Config, cl client.Client) (err error) {
	if c.Address, err = GetURL(context.TODO(), cl, c.Address); err != nil {
		return err
	}

	ca, err := RetrieveAPIServerCA(restcfg, []byte{}, c.TrustedCA)
	if err != nil {
		return err
	}
	c.CA = base64.StdEncoding.EncodeToString(ca)
	return nil
}

// RetrieveAPIServerCA retrieves the APIServerCA, either from the CAData in the restConfig, or reading from the CAFile.
func RetrieveAPIServerCA(restcfg *rest.Config, caOverride []byte, trustedCA bool) ([]byte, error) {
	if len(caOverride) > 0 {
		return caOverride, nil
	}

	// if the CA is trusted, we don't need to advertise it. All the peering clusters should trust it.
	if trustedCA {
		return []byte{}, nil
	}

	if restcfg.CAData != nil && len(restcfg.CAData) > 0 {
		return restcfg.CAData, nil
	}
	if restcfg.CAFile != "" {
		// CAData is not available, read it from the CAFile.
		data, err := os.ReadFile(restcfg.CAFile)
		if err != nil {
			return []byte{}, err
		}
		return data, nil
	}
	return []byte{}, nil
}
