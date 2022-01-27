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

package move

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func restoreSnapshot(ctx context.Context, cl client.Client,
	oldPvc, newPvc *corev1.PersistentVolumeClaim, nodeName, resticRepositoryURL, resticPassword string) error {
	job, err := createRestorerJob(ctx, cl, oldPvc, newPvc, nodeName, resticRepositoryURL, resticPassword)
	if err != nil {
		return err
	}

	return waitForJob(ctx, cl, job)
}

func createRestorerJob(ctx context.Context, cl client.Client,
	oldPvc, newPvc *corev1.PersistentVolumeClaim,
	nodeName, resticRepositoryURL, resticPassword string) (*batchv1.Job, error) {
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "liqo-restorer-",
			Namespace:    oldPvc.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32Ptr(10),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "kubernetes.io/hostname",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{nodeName},
											},
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "restic",
							Image:           resticImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"-r",
								fmt.Sprintf("%s%s", resticRepositoryURL, oldPvc.GetUID()),
								"restore", "latest",
								"--target", "/restore",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "RESTIC_PASSWORD",
									Value: resticPassword,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "restore",
									MountPath: "/restore",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Volumes: []corev1.Volume{
						{
							Name: "restore",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: newPvc.GetName(),
								},
							},
						},
					},
				},
			},
		},
	}

	if err := cl.Create(ctx, &job); err != nil {
		return nil, err
	}
	return &job, nil
}
