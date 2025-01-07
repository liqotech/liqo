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

package util

import (
	"context"
	"os"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

var (
	image = "nginx"
)

func init() {
	// get the DOCKER_PROXY variable from the environment, if set.
	dockerProxy, ok := os.LookupEnv("DOCKER_PROXY")
	if ok {
		image = dockerProxy + "/" + image
	}
}

// DeploymentOption is a function that modifies a Deployment.
type DeploymentOption func(*appsv1.Deployment)

// LocalDeploymentOption sets the Deployment to be scheduled on local nodes.
func LocalDeploymentOption() DeploymentOption {
	return func(deploy *appsv1.Deployment) {
		deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      consts.TypeLabel,
									Operator: corev1.NodeSelectorOpDoesNotExist,
								},
							},
						},
					},
				},
			},
		}
	}
}

// RemoteDeploymentOption sets the Deployment to be scheduled on remote nodes.
func RemoteDeploymentOption() DeploymentOption {
	return func(deploy *appsv1.Deployment) {
		deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      consts.TypeLabel,
									Operator: corev1.NodeSelectorOpExists,
								},
							},
						},
					},
				},
			},
		}
	}
}

// RuntimeClassOption sets the RuntimeClass of the Deployment.
func RuntimeClassOption(runtimeClass string) DeploymentOption {
	return func(deploy *appsv1.Deployment) {
		deploy.Spec.Template.Spec.RuntimeClassName = &runtimeClass
	}
}

// EnforceDeployment creates or updates a Deployment with the given name in the given namespace.
func EnforceDeployment(ctx context.Context, cl client.Client, namespace, name string, options ...DeploymentOption) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	return Second(resource.CreateOrUpdate(ctx, cl, deploy, func() error {
		deploy.Spec.Replicas = ptr.To(int32(1))
		deploy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": name},
		}
		deploy.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": name},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:            name,
						Image:           image,
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				},
			},
		}

		for _, opt := range options {
			opt(deploy)
		}

		return nil
	}))
}

// EnsureDeploymentDeletion deletes a Deployment with the given name in the given namespace.
func EnsureDeploymentDeletion(ctx context.Context, cl client.Client, namespace, name string) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	return client.IgnoreNotFound(cl.Delete(ctx, deploy))
}

// GetPodsFromDeployment returns the Pods of a Deployment with the given name in the given namespace.
func GetPodsFromDeployment(ctx context.Context, cl client.Client, namespace, name string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := cl.List(ctx, &pods, client.InNamespace(namespace), client.MatchingLabels{"app": name}); err != nil {
		return nil, err
	}

	return pods.Items, nil
}

// AddArgumentToDeployment adds an argument to the containers of a Deployment with the given name in the given namespace.
func AddArgumentToDeployment(ctx context.Context, cl client.Client, namespace, name, argument string, index int) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(deploy), deploy); err != nil {
		return err
	}

	deploy.Spec.Template.Spec.Containers[index].Args = append(deploy.Spec.Template.Spec.Containers[index].Args, argument)

	return cl.Update(ctx, deploy)
}

// RemoveArgumentFromDeployment removes an argument from the containers of a Deployment with the given name in the given namespace.
func RemoveArgumentFromDeployment(ctx context.Context, cl client.Client, namespace, name, argument string, index int) error {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(deploy), deploy); err != nil {
		return err
	}

	container := &deploy.Spec.Template.Spec.Containers[index]
	for j, arg := range container.Args {
		if arg == argument {
			deploy.Spec.Template.Spec.Containers[index].Args = slices.Delete(deploy.Spec.Template.Spec.Containers[index].Args, j, j+1)
			break
		}
	}

	return cl.Update(ctx, deploy)
}
