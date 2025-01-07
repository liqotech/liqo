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

package client

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pterm/pterm"
)

// HTTPClient struct to handle http requests.
type HTTPClient struct {
	Client *http.Client
}

// NewHTTPClient creates a new HttpClient with a specified timeout.
func NewHTTPClient(timeout time.Duration) *HTTPClient {
	return &HTTPClient{
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Curl executes a curl command to the specified target.
func (hc *HTTPClient) Curl(ctx context.Context, url string, quiet bool, logger *pterm.Logger) (ok bool, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		logger.Warn("Failed to create HTTP request", logger.Args(
			"target", url, "error", err.Error(),
		))
		return false, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := hc.Client.Do(req)
	if err != nil {
		logger.Warn("Curl command failed", logger.Args(
			"target", url, "error", err.Error(),
		))
		return false, fmt.Errorf("failed to execute curl command: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		if !quiet {
			logger.Info("Curl command successful", logger.Args(
				"target", url,
			))
		}
		return true, nil
	}
	logger.Warn("Curl command failed", logger.Args(
		"target", url,
	))

	return false, nil
}
