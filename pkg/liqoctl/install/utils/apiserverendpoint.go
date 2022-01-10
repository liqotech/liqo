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

package installutils

import (
	"fmt"
	"net/url"
	"strings"

	"k8s.io/client-go/rest"
)

// CheckEndpoint checks if the provided endpoint end the endpoint contained in the rest.Config
// point to the same server.
func CheckEndpoint(endpoint string, config *rest.Config) (bool, error) {
	configHostname, configPort, err := getHostnamePort(config.Host)
	if err != nil {
		return false, err
	}

	endpointHostname, endpointPort, err := getHostnamePort(endpoint)
	if err != nil {
		return false, err
	}

	return configHostname == endpointHostname && configPort == endpointPort, nil
}

func getHostnamePort(urlString string) (hostname, port string, err error) {
	if !strings.HasPrefix(urlString, "https://") {
		urlString = fmt.Sprintf("https://%v", urlString)
	}

	var parsedURL *url.URL
	parsedURL, err = url.Parse(urlString)
	if err != nil {
		return "", "", err
	}

	hostname = parsedURL.Hostname()
	port = parsedURL.Port()
	defaultPort(&port)

	return hostname, port, nil
}

func defaultPort(port *string) {
	if port != nil && *port == "" {
		*port = "443"
	}
}
