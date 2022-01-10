// Copyright 2019-2022 The Liqo Authors
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
	"time"

	"github.com/gruntwork-io/terratest/modules/k8s"
	"github.com/onsi/ginkgo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/consts"
	testutils "github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	retries             = 60
	sleepBetweenRetries = 3 * time.Second

	// StatefulSetName is the name of the test StatefulSet.
	StatefulSetName = "liqo-storage"
)

// DeployApp creates the namespace and deploys the applications. It returns an error in case of failures.
func DeployApp(ctx context.Context, config *rest.Config, namespace string) error {
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespace,
			Labels: testutils.GetNamespaceLabel(true),
		},
	}

	if err = cl.Create(ctx, ns); err != nil {
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
			Replicas: pointer.Int32(2),
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
							Name:  "tester",
							Image: "nginx",
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
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("25Mi"),
							},
						},
						StorageClassName: pointer.StringPtr("liqo"),
					},
				},
			},
		},
	}

	return cl.Create(ctx, statefulSet)
}

// WaitDemoApp waits until each pod in the StatefulSet is ready.
func WaitDemoApp(t ginkgo.GinkgoTInterface, options *k8s.KubectlOptions) {
	k8s.WaitUntilNumPodsCreated(t, options, metav1.ListOptions{}, 2, retries, sleepBetweenRetries)

	pods := k8s.ListPods(t, options, metav1.ListOptions{})
	for index := range pods {
		k8s.WaitUntilPodAvailable(t, options, pods[index].Name, retries, sleepBetweenRetries)
	}
}
