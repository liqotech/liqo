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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
)

// CreateAllDeployments creates all the deployments.
func CreateAllDeployments(ctx context.Context, cl *client.Client) (totreplicas int32, err error) {
	replicas, err := getReplicas(ctx, cl.Consumer)
	if err != nil {
		return 0, fmt.Errorf("consumer error getting replicas: %w", err)
	}
	totreplicas += replicas
	if err := CreateDeployment(ctx, cl.Consumer, replicas, cl.ConsumerName, false); err != nil {
		return 0, fmt.Errorf("consumer error creating deployment: %w", err)
	}
	if err := CreateDeployment(ctx, cl.Consumer, replicas, cl.ConsumerName, true); err != nil {
		return 0, fmt.Errorf("consumer error creating deployment: %w", err)
	}
	for k := range cl.Providers {
		replicas, err := getReplicas(ctx, cl.Providers[k])
		if err != nil {
			return 0, fmt.Errorf("provider %q error getting replicas: %w", k, err)
		}
		totreplicas += replicas
		if err := CreateDeployment(ctx, cl.Consumer, replicas, k, false); err != nil {
			return 0, fmt.Errorf("provider %q error creating deployment: %w", k, err)
		}
		if err := CreateDeployment(ctx, cl.Consumer, replicas, k, true); err != nil {
			return 0, fmt.Errorf("consumer error creating deployment: %w", err)
		}
	}

	if err := WaitDeploymentReady(ctx, cl.Consumer, cl.ConsumerName, false); err != nil {
		return 0, fmt.Errorf("consumer error waiting for deployments to be ready: %w", err)
	}
	if err := WaitDeploymentReady(ctx, cl.Consumer, cl.ConsumerName, true); err != nil {
		return 0, fmt.Errorf("consumer error waiting for deployments to be ready: %w", err)
	}
	for k := range cl.Providers {
		if err := WaitDeploymentReady(ctx, cl.Consumer, k, false); err != nil {
			return 0, fmt.Errorf("provider %q error waiting for deployments to be ready: %w", k, err)
		}
		if err := WaitDeploymentReady(ctx, cl.Consumer, k, true); err != nil {
			return 0, fmt.Errorf("provider %q error waiting for deployments to be ready: %w", k, err)
		}
	}
	return totreplicas, nil
}

// CreateDeployment creates a deployment.
func CreateDeployment(ctx context.Context, cl ctrlclient.Client, replicas int32, suffix string, hostnetwork bool) error {
	deploymentName := DeploymentName
	ports := []corev1.ContainerPort{{ContainerPort: 80}}
	if hostnetwork {
		deploymentName = DeploymentName + "-host"
		ports = []corev1.ContainerPort{}
	}
	dp := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName + "-" + suffix,
			Namespace: NamespaceName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{
				PodLabelAppCluster: deploymentName + "-" + suffix,
			}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						PodLabelApp:        deploymentName,
						PodLabelAppCluster: deploymentName + "-" + suffix,
					},
				},
				Spec: corev1.PodSpec{
					HostNetwork: hostnetwork,
					Containers: []corev1.Container{
						{
							Name:    "netshoot",
							Image:   "ghcr.io/nicolaka/netshoot",
							Command: []string{"python3", "-m", "http.server", "80"},
							Ports:   ports},
					},
					NodeSelector: map[string]string{consts.RemoteClusterID: suffix},
				},
			},
		},
	}
	if err := cl.Create(ctx, dp); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
		return err
	}
	return nil
}

// WaitDeploymentReady waits for the deployment to be ready.
func WaitDeploymentReady(ctx context.Context, cl ctrlclient.Client, suffix string, hostnetwork bool) error {
	deploymentName := DeploymentName
	if hostnetwork {
		deploymentName = DeploymentName + "-host"
	}
	if err := wait.PollUntilContextCancel(ctx, time.Second*2, true, func(ctx context.Context) (done bool, err error) {
		dp := &appsv1.Deployment{}
		if err := cl.Get(ctx, ctrlclient.ObjectKey{Namespace: NamespaceName, Name: deploymentName + "-" + suffix}, dp); err != nil {
			return false, err
		}
		if dp.Status.ReadyReplicas == *dp.Spec.Replicas {
			return true, nil
		}
		return false, nil
	}); err != nil {
		return fmt.Errorf("error waiting for deployment to be ready: %w", err)
	}
	return nil
}
