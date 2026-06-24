// Copyright 2019-2026 The Liqo Authors
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

package check

import (
	"context"
	"fmt"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

// RunChecksNodeToPod runs the checks from the nodes to the pods.
func RunChecksNodeToPod(ctx context.Context, cl *client.Client, cfg client.Configs, opts *flags.Options,
	totreplicas int32) (successCount, errorCount int32, err error) {
	var successCountTot, errorCountTot int32

	targets, err := ForgePodTargets(ctx, cl, totreplicas)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to forge targets: %w", err)
	}

	consumerPods, err := listPods(ctx, cl.Consumer, cl.ConsumerName, true)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list consumer pods: %w", err)
	}
	successCount, errorCount, err = RunCheckToTargets(ctx, consumerPods, cfg[cl.ConsumerName],
		opts, targets[cl.ConsumerName], ExecCurl)
	if err != nil {
		return 0, 0, fmt.Errorf("consumer failed to run checks: %w", err)
	}
	successCountTot += successCount
	errorCountTot += errorCount

	for k := range cl.Providers {
		providerPods, err := listPods(ctx, cl.Providers[k], k, true)
		if err != nil {
			return successCountTot, errorCountTot, fmt.Errorf("failed to list provider %q pods: %w", k, err)
		}
		successCount, errorCount, err := RunCheckToTargets(ctx, providerPods,
			cfg[k], opts, targets[k], ExecCurl)
		if err != nil {
			return 0, 0, fmt.Errorf("provider %q failed to run checks: %w", k, err)
		}
		successCountTot += successCount
		errorCountTot += errorCount
	}

	return successCountTot, errorCountTot, nil
}
