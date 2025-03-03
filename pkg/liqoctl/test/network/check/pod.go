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

package check

import (
	"context"
	"fmt"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

// RunChecksPodToPod runs the checks from the pods to the pods.
func RunChecksPodToPod(ctx context.Context, cl *client.Client, cfg client.Configs, opts *flags.Options,
	totreplicas int32) (successCount, errorCount int32, err error) {
	var successCountTot, errorCountTot int32

	targets, err := ForgePodTargets(ctx, cl, totreplicas)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to forge targets: %w", err)
	}

	successCount, errorCount, err = RunCheckToTargets(ctx, cl.Consumer, cfg[cl.ConsumerName],
		opts, cl.ConsumerName, targets[cl.ConsumerName], false, ExecCurl)
	successCountTot += successCount
	errorCountTot += errorCount
	if err != nil {
		return successCountTot, errorCountTot, fmt.Errorf("consumer failed to run checks: %w", err)
	}

	for k := range cl.Providers {
		successCount, errorCount, err := RunCheckToTargets(ctx, cl.Providers[k], cfg[k],
			opts, k, targets[k], false, ExecCurl)
		successCountTot += successCount
		errorCountTot += errorCount
		if err != nil {
			return successCountTot, errorCountTot, fmt.Errorf("provider %q failed to run checks: %w", k, err)
		}
	}

	return successCountTot, errorCountTot, nil
}
