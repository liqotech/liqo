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

package apiserver

import (
	"bytes"
	"context"
	"fmt"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// JobName -> the name of the tester job.
	JobName = "kubectl"

	containerName      = "kubectl"
	serviceAccountName = "kubectl"
	clusterRoleName    = "admin"
)

var (
	image = "bitnami/kubectl"
)

func init() {
	// get the DOCKER_PROXY variable from the environment, if set.
	dockerProxy, ok := os.LookupEnv("DOCKER_PROXY")
	if ok {
		image = dockerProxy + "/" + image
	}
}

// CreateKubectlJob creates the offloaded kubectl job to perform a request on the remote API server.
func CreateKubectlJob(ctx context.Context, cl client.Client, namespace string, v *version.Info) error {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: JobName, Namespace: namespace},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:      containerName,
						Image:     fmt.Sprintf("%s:%s.%s", image, v.Major, v.Minor),
						Args:      []string{"get", "pods", "-n", namespace, "--no-headers", "-o", "custom-columns=:.metadata.name"},
						Resources: util.ResourceRequirements(),
					}},
					ServiceAccountName: serviceAccountName,
					RestartPolicy:      corev1.RestartPolicyNever,
				},
			},
		},
	}

	return cl.Create(ctx, job)
}

// RetrieveJobLogs retrieves the logs from the offloaded kubectl pod.
func RetrieveJobLogs(ctx context.Context, cl kubernetes.Interface, namespace string) (podName, retrieved string, err error) {
	pods, err := cl.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil || len(pods.Items) != 1 {
		return "", "", fmt.Errorf("failed to retrieve pods: %w", err)
	}

	logOpts := corev1.PodLogOptions{
		Container: containerName,
	}

	stream, err := cl.CoreV1().Pods(namespace).GetLogs(pods.Items[0].GetName(), &logOpts).Stream(ctx)
	if err != nil {
		return "", "", fmt.Errorf("could not get stream from logs request: %w", err)
	}

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(stream); err != nil {
		return "", "", fmt.Errorf("could not read from logs stream: %w", err)
	}
	return pods.Items[0].GetName(), buffer.String(), nil
}

// CreateServiceAccount creates the service account leveraged by the kubectl pod.
func CreateServiceAccount(ctx context.Context, cl client.Client, namespace string) error {
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace}}
	return cl.Create(ctx, sa)
}

// CreateRoleBinding creates the role binding granting the appropriate permissions to the service account.
func CreateRoleBinding(ctx context.Context, cl client.Client, namespace string) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName, Namespace: namespace},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: clusterRoleName, APIGroup: rbacv1.GroupName},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: serviceAccountName, Namespace: namespace}},
	}
	return cl.Create(ctx, rb)
}
