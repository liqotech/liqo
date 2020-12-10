package utils

import (
	"crypto/tls"
	"crypto/x509"
	goerrors "errors"
	"fmt"
	"github.com/liqotech/liqo/pkg/discovery"
	"net/http"
)

// check if the error is due to a TLS certificate signed by unknown authority
func IsUnknownAuthority(err error) bool {
	var err509 x509.UnknownAuthorityError
	var err509Hostname x509.HostnameError
	return goerrors.As(err, &err509) || goerrors.As(err, &err509Hostname)
}

// contact the remote cluster to get its info
// it returns also if the remote cluster exposes a trusted certificate
func GetClusterInfo(url string) (*http.Response, discovery.TrustMode, error) {
	trustMode := discovery.TrustModeTrusted
	tr := &http.Transport{}
	client := &http.Client{Transport: tr}
	resp, err := client.Get(fmt.Sprintf("%s/ids", url))
	if IsUnknownAuthority(err) {
		trustMode = discovery.TrustModeUntrusted
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: tr}
		resp, err = client.Get(fmt.Sprintf("%s/ids", url))
	}
	if err != nil {
		return nil, discovery.TrustModeUnknown, err
	}
	return resp, trustMode, nil
}
