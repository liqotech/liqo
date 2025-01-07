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

package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/klog/v2"
)

// Send sends the telemetry item to the Liqo telemetry server.
func Send(ctx context.Context, endpoint string, item *Telemetry, timeout time.Duration) error {
	body, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal telemetry item: %w", err)
	}

	buff := bytes.NewBuffer(body)
	httpClient := http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, buff)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telemetry item: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		message, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to send telemetry item: %w", err)
		}
		return fmt.Errorf("unexpected status code: %d: %s", resp.StatusCode, message)
	}
	if err = resp.Body.Close(); err != nil {
		return fmt.Errorf("failed to close response body: %w", err)
	}

	prettyMarshal, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		klog.Errorf("failed to marshal telemetry item: %s", err)
		return nil
	}

	klog.Infof("successfully sent telemetry item %s to %s", string(prettyMarshal), endpoint)
	return nil
}
