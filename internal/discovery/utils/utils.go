// Package utils contains functions useful for the discovery component,
// in particular during the communications with a remote cluster.
package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	goerrors "errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
)

// IsUnknownAuthority checks if the error is due to a TLS certificate signed by unknown authority.
func IsUnknownAuthority(err error) bool {
	var err509 x509.UnknownAuthorityError
	var err509Hostname x509.HostnameError
	return goerrors.As(err, &err509) || goerrors.As(err, &err509Hostname)
}

// GetClusterInfo contacts the remote cluster to get its info,
// it returns also if the remote cluster exposes a trusted certificate.
func GetClusterInfo(skipTLSVerify bool, url string) (*auth.ClusterInfo, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
	}
	client := &http.Client{Transport: tr}
	resp, err := httpGet(context.TODO(), client, fmt.Sprintf("%s%s", url, auth.IdsURI))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	var ids auth.ClusterInfo
	if err = json.Unmarshal(respBytes, &ids); err != nil {
		klog.Error(err)
		return nil, err
	}

	return &ids, nil
}

func httpGet(ctx context.Context, client *http.Client, url string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return client.Do(req)
}
