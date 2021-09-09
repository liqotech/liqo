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

// Package utils contains functions useful for the discovery component,
// in particular during the communications with a remote cluster.
package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
)

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
