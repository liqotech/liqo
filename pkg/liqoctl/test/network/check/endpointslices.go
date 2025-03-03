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
	"time"

	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
)

// Targets ia a map where the key is the name of the provider/consumer
// and the values are the endpoints from their point of view.
type Targets map[string][]string

// ForgePodTargets creates a map of targets for the pod-to-pod tests.
func ForgePodTargets(ctx context.Context, cl *client.Client, totalReplicas int32) (Targets, error) {
	var target Targets = make(map[string][]string)

	if err := ForgePodTargetForConsumer(ctx, cl, totalReplicas, target); err != nil {
		return nil, err
	}

	for k := range cl.Providers {
		if err := ForgePodTargetForProvider(ctx, cl, k, totalReplicas, target); err != nil {
			return nil, err
		}
	}
	return target, nil
}

// ForgePodTargetForProvider creates a target for a specific cluster.
func ForgePodTargetForProvider(ctx context.Context, cl *client.Client, name string, totalReplicas int32, target Targets) error {
	eps := discoveryv1.EndpointSliceList{}

	timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(timeout, 5*time.Second, true, func(ctx context.Context) (done bool, err error) {
		if err := cl.Providers[name].List(ctx, &eps,
			ctrlclient.InNamespace(setup.NamespaceName),
			ctrlclient.MatchingLabels{
				discoveryv1.LabelServiceName: setup.DeploymentName,
			},
		); err != nil {
			return false, err
		}
		return len(eps.Items) == 2, nil
	}); err != nil {
		if len(eps.Items) != 2 {
			return fmt.Errorf("%q expected 2 endpoint slice, got %d", name, len(eps.Items))
		}
		return fmt.Errorf("error waiting for provider %q endpoint slice: %w", name, err)
	}

	if len(eps.Items[0].Endpoints)+len(eps.Items[1].Endpoints) != int(totalReplicas) {
		return fmt.Errorf("%q expected %d endpoints, got %d", name, totalReplicas, len(eps.Items[0].Endpoints)+len(eps.Items[1].Endpoints))
	}

	target[name] = make([]string, len(eps.Items[0].Endpoints)+len(eps.Items[1].Endpoints))
	for i := range eps.Items[0].Endpoints {
		target[name][i] = eps.Items[0].Endpoints[i].Addresses[0]
	}

	for i := range eps.Items[1].Endpoints {
		target[name][i+len(eps.Items[0].Endpoints)] = eps.Items[1].Endpoints[i].Addresses[0]
	}
	return nil
}

// ForgePodTargetForConsumer creates a target for the consumer cluster.
func ForgePodTargetForConsumer(ctx context.Context, cl *client.Client, totalReplicas int32, target Targets) error {
	eps := discoveryv1.EndpointSliceList{}

	timeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := wait.PollUntilContextCancel(timeout, 5*time.Second, true, func(ctx context.Context) (done bool, err error) {
		if err := cl.Consumer.List(ctx, &eps,
			ctrlclient.InNamespace(setup.NamespaceName),
			ctrlclient.MatchingLabels{
				discoveryv1.LabelServiceName: setup.DeploymentName,
			},
		); err != nil {
			return false, err
		}
		return len(eps.Items) == 1, nil
	}); err != nil {
		if len(eps.Items) != 1 {
			return fmt.Errorf("consumer expected 1 endpoint slice, got %d", len(eps.Items))
		}
		return fmt.Errorf("error waiting for consumer endpoint slice: %w", err)
	}

	if len(eps.Items[0].Endpoints) != int(totalReplicas) {
		return fmt.Errorf("consumer expected %d endpoints, got %d", totalReplicas, len(eps.Items[0].Endpoints))
	}

	target[cl.ConsumerName] = make([]string, len(eps.Items[0].Endpoints))
	for i := range eps.Items[0].Endpoints {
		target[cl.ConsumerName][i] = eps.Items[0].Endpoints[i].Addresses[0]
	}
	return nil
}
