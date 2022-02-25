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

package common

import (
	"fmt"
	"net"
	"strings"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/liqotech/liqo/pkg/utils/restcfg"
)

// WireGuardConfig holds the WireGuard configuration.
type WireGuardConfig struct {
	PubKey       string
	EndpointIP   string
	EndpointPort string
	BackEndType  string
}

// GetLiqoctlRestConf gets a valid REST config and set a default value for the RateLimiters.
func GetLiqoctlRestConf() (*rest.Config, error) {
	restConfig, err := config.GetConfig()
	if err != nil {
		if strings.HasSuffix(err.Error(), clientcmd.ErrEmptyConfig.Error()) {
			// Rewrite the error message (you likely want KUBECONFIG rather than KUBERNETES_MASTER)
			return nil, fmt.Errorf("no configuration provided, please set the environment variable KUBECONFIG")
		}
		return nil, err
	}
	return restcfg.SetRateLimiter(restConfig), nil
}

// ExtractValueFromArgumentList extracts the argument value from an argument list.
func ExtractValueFromArgumentList(key string, argumentList []string) (string, error) {
	prefix := key + "="
	for _, argument := range argumentList {
		if strings.HasPrefix(argument, prefix) {
			return strings.Join(strings.Split(argument, "=")[1:], "="), nil
		}
	}
	return "", fmt.Errorf("argument not found")
}

// getFreePort get a free port on the system by listening in a socket,
// checking the bound port number and then closing the socket.
func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
