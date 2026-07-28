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
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
	utilspod "github.com/liqotech/liqo/pkg/utils/pod"
)

const (
	// targetsStabilityTimeout is the maximum time waited for the endpoints to settle.
	targetsStabilityTimeout = 60 * time.Second
	// targetsStabilityInterval is the delay between two consecutive readings of the endpoints.
	targetsStabilityInterval = 3 * time.Second
)

// Targets ia a map where the key is the name of the provider/consumer
// and the values are the endpoints from their point of view.
type Targets map[string][]string

// Equal returns whether the two sets contain the same addresses for the same clusters.
func (t Targets) Equal(other Targets) bool {
	if len(t) != len(other) {
		return false
	}
	for name := range t {
		o, ok := other[name]
		if !ok || len(t[name]) != len(o) {
			return false
		}
		current, previous := slices.Clone(t[name]), slices.Clone(o)
		slices.Sort(current)
		slices.Sort(previous)
		if !slices.Equal(current, previous) {
			return false
		}
	}
	return true
}

// ForgePodTargets returns the addresses to probe in the pod-to-pod tests, once they are stable and
// backed by running pods.
//
// The endpoints are read repeatedly, and are accepted only once two consecutive readings agree and
// are consistent with the pods (see ValidateTargets). An EndpointSlice read while the pods are being
// replaced exposes the address of a pod which is already gone: the checks would then probe a dead
// address until they time out, reporting a connectivity failure which is not one.
func ForgePodTargets(ctx context.Context, cl *client.Client, totalReplicas int32) (Targets, error) {
	var previous Targets
	var lastErr error

	timeout, cancel := context.WithTimeout(ctx, targetsStabilityTimeout)
	defer cancel()

	if err := wait.PollUntilContextCancel(timeout, targetsStabilityInterval, true,
		func(ctx context.Context) (done bool, err error) {
			current, ferr := forgePodTargets(ctx, cl, totalReplicas)
			if ferr != nil {
				previous, lastErr = nil, ferr
				return false, nil
			}

			if verr := ValidateTargets(ctx, cl, current); verr != nil {
				previous, lastErr = nil, verr
				return false, nil
			}

			if previous != nil && previous.Equal(current) {
				return true, nil
			}

			previous, lastErr = current, fmt.Errorf("the endpoints are still changing")
			return false, nil
		}); err != nil {
		return nil, fmt.Errorf("the endpoints did not settle within %s: %w", targetsStabilityTimeout, lastErr)
	}

	return previous, nil
}

// ValidateTargets checks the addresses to probe against the pods of the cluster they belong to.
//
// The consumer has a pod for every backend: the local ones, and the offloaded ones, whose status
// carries the address remapped by the virtual kubelet. A provider, instead, only hosts part of them:
// the remaining addresses are reflected from the other clusters and have no pod to be resolved
// against. The check is therefore expressed as "every running and ready pod of a cluster must appear
// among its targets": an address left behind by a pod which has been replaced makes the pod that
// replaced it missing from the list, and is detected.
func ValidateTargets(ctx context.Context, cl *client.Client, targets Targets) error {
	if err := validateClusterTargets(ctx, cl.Consumer, cl.ConsumerName, targets[cl.ConsumerName]); err != nil {
		return err
	}

	for name := range cl.Providers {
		if err := validateClusterTargets(ctx, cl.Providers[name], name, targets[name]); err != nil {
			return err
		}
	}

	return nil
}

func validateClusterTargets(ctx context.Context, cl ctrlclient.Client, name string, targets []string) error {
	pods := corev1.PodList{}
	if err := cl.List(ctx, &pods,
		ctrlclient.InNamespace(setup.NamespaceName),
		ctrlclient.MatchingLabels{setup.PodLabelApp: setup.DeploymentName},
	); err != nil {
		return fmt.Errorf("failed to list the pods of cluster %q: %w", name, err)
	}

	targetSet := make(map[string]any, len(targets))
	for i := range targets {
		targetSet[targets[i]] = nil
	}

	var missing []string
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.PodIP == "" {
			continue
		}

		_, isTarget := targetSet[pod.Status.PodIP]
		ready, reason := utilspod.IsPodReady(pod)
		terminating := pod.DeletionTimestamp != nil

		switch {
		case isTarget && terminating:
			return fmt.Errorf("target %s of cluster %q is backed by pod %s, which is terminating",
				pod.Status.PodIP, name, pod.Name)
		case isTarget && !ready:
			return fmt.Errorf("target %s of cluster %q is backed by pod %s, which is not ready: %s",
				pod.Status.PodIP, name, pod.Name, reason)
		case !isTarget && ready && !terminating:
			missing = append(missing, fmt.Sprintf("%s (%s)", pod.Name, pod.Status.PodIP))
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("cluster %q: pods %s are running and ready, but their addresses are not among the endpoints %v",
			name, strings.Join(missing, ", "), targets)
	}

	return nil
}

// RefreshConsumerTargets re-reads the addresses to probe from the consumer.
func RefreshConsumerTargets(ctx context.Context, cl *client.Client, totalReplicas int32) ([]string, error) {
	target := Targets{}
	if err := ForgePodTargetForConsumer(ctx, cl, totalReplicas, target); err != nil {
		return nil, err
	}
	return target[cl.ConsumerName], nil
}

// RefreshProviderTargets re-reads the addresses to probe from the given provider.
func RefreshProviderTargets(ctx context.Context, cl *client.Client, name string, totalReplicas int32) ([]string, error) {
	target := Targets{}
	if err := ForgePodTargetForProvider(ctx, cl, name, totalReplicas, target); err != nil {
		return nil, err
	}
	return target[name], nil
}

// forgePodTargets creates a map of targets for the pod-to-pod tests.
func forgePodTargets(ctx context.Context, cl *client.Client, totalReplicas int32) (Targets, error) {
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
