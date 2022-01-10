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

// Package utils contains functions useful for the discovery component,
// in particular during the communications with a remote cluster.
package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
)

const (
	// HTTPRequestTimeout is the timeout used in http connection to a remote authentication service.
	HTTPRequestTimeout = 5 * time.Second
)

// GetClusterInfo contacts the remote cluster to get its info,
// it returns also if the remote cluster exposes a trusted certificate.
func GetClusterInfo(ctx context.Context, transport *http.Transport, url string) (*auth.ClusterInfo, error) {
	client := &http.Client{
		Transport: transport,
		Timeout:   HTTPRequestTimeout,
	}
	resp, err := httpGet(ctx, client, fmt.Sprintf("%s%s", url, auth.IdsURI))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return client.Do(req)
}
