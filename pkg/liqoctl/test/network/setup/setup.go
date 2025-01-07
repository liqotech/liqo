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

package setup

import (
	"context"
	"fmt"
	"time"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/flags"
)

// MakeInfrastructure sets up the infrastructure for the network tests.
func MakeInfrastructure(ctx context.Context, cl *client.Client, opts *flags.Options) (totreplicas int32, err error) {
	if err := AddConsumerNodeLabels(ctx, cl); err != nil {
		return 0, fmt.Errorf("error adding consumer node labels: %w", err)
	}

	if err := CreateNamespace(ctx, cl); err != nil {
		return 0, fmt.Errorf("error creating namespace: %w", err)
	}

	if err := OffloadNamespace(ctx, cl); err != nil {
		return 0, fmt.Errorf("error offloading namespace: %w", err)
	}

	if err := CreatePolicy(ctx, cl); err != nil {
		return 0, fmt.Errorf("error creating policy: %w", err)
	}

	if totreplicas, err = CreateAllDeployments(ctx, cl); err != nil {
		return 0, fmt.Errorf("error creating deployments: %w", err)
	}

	if err := CreateService(ctx, cl, opts); err != nil {
		return 0, fmt.Errorf("error creating service: %w", err)
	}

	if opts.IPRemapping {
		if err := CreateAllIP(ctx, cl); err != nil {
			return 0, fmt.Errorf("error creating ip: %w", err)
		}
	}

	// sleep for a while to let the network be ready
	time.Sleep(5 * time.Second)

	return totreplicas, nil
}
