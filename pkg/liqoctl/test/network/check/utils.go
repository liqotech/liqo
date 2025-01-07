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

	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/liqoctl/test/network/setup"
)

// listPods lists the pods.
func listPods(ctx context.Context, cl ctrlclient.Client, owner string, hostnetwork bool) (*corev1.PodList, error) {
	deploymentName := setup.DeploymentName
	if hostnetwork {
		deploymentName = setup.DeploymentName + "-host"
	}
	pods := corev1.PodList{}
	if err := cl.List(ctx, &pods,
		ctrlclient.InNamespace(setup.NamespaceName),
		ctrlclient.MatchingLabels{setup.PodLabelAppCluster: deploymentName + "-" + owner},
	); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	return &pods, nil
}
