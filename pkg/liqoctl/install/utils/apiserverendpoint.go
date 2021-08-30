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
