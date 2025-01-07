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

package storage

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	testutils "github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	retries             = 60
	sleepBetweenRetries = 3 * time.Second

	// StatefulSetName is the name of the test StatefulSet.
	StatefulSetName = "liqo-storage"
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

// DeployApp creates the namespace and deploys the applications. It returns an error in case of failures.
func DeployApp(ctx context.Context, cluster *tester.ClusterContext, namespace string, replicas int32) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if err := cluster.ControllerClient.Create(ctx, ns); err != nil {
		return err
	}

	if err := testutils.OffloadNamespace(cluster.KubeconfigPath, namespace); err != nil {
		return err
	}

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      StatefulSetName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": StatefulSetName,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": StatefulSetName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": StatefulSetName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:      "tester",
							Image:     image,
							Resources: testutils.ResourceRequirements(),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "liqo-storage-claim",
									MountPath: "/usr/share/nginx/html",
								},
							},
						},
					},
					// put pods in anti-affinity, and prefer local cluster. In this way one pod will be local, the other remote
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
								{
									Weight: 2,
									Preference: corev1.NodeSelectorTerm{
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
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app": StatefulSetName,
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "liqo-storage-claim",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("25Mi"),
							},
						},
						StorageClassName: pointer.String("liqo"),
					},
				},
			},
		},
	}

	return cluster.ControllerClient.Create(ctx, statefulSet)
}

// ScaleStatefulSet scales the StatefulSet to the desired number of replicas.
func ScaleStatefulSet(ctx context.Context, t ginkgo.GinkgoTInterface, options *k8s.KubectlOptions,
	cl kubernetes.Interface, namespace string, replicas int32) error {
	statefulSet, err := cl.AppsV1().StatefulSets(namespace).Get(ctx, StatefulSetName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	statefulSet.Spec.Replicas = &replicas
	_, err = cl.AppsV1().StatefulSets(namespace).Update(ctx, statefulSet, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	WaitDemoApp(t, options, int(replicas))
	return nil
}

// WaitDemoApp waits until each pod in the StatefulSet is ready.
func WaitDemoApp(t ginkgo.GinkgoTInterface, options *k8s.KubectlOptions, replicas int) {
	k8s.WaitUntilNumPodsCreated(t, options, metav1.ListOptions{
		LabelSelector: "app=" + StatefulSetName,
	}, replicas, retries, sleepBetweenRetries)

	pods := k8s.ListPods(t, options, metav1.ListOptions{
		LabelSelector: "app=" + StatefulSetName,
	})
	for index := range pods {
		k8s.WaitUntilPodAvailable(t, options, pods[index].Name, retries, sleepBetweenRetries)
	}
}

// WriteToVolume writes a file to the volume of the StatefulSet.
func WriteToVolume(ctx context.Context, cl kubernetes.Interface, config *rest.Config, namespace string) error {
	_, _, err := testutils.ExecCmd(ctx, config, cl, fmt.Sprintf("%s-0", StatefulSetName), namespace,
		"echo -n test > /usr/share/nginx/html/index.html")
	return err
}

// ReadFromVolume reads a file from the volume of the StatefulSet.
func ReadFromVolume(ctx context.Context,
	cl kubernetes.Interface, config *rest.Config, namespace string) (string, error) {
	out, _, err := testutils.ExecCmd(ctx, config, cl, fmt.Sprintf("%s-0", StatefulSetName), namespace,
		"cat /usr/share/nginx/html/index.html")
	return out, err
}
