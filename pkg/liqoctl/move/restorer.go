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

package move

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (o *Options) restoreSnapshot(ctx context.Context,
	oldPvc, newPvc *corev1.PersistentVolumeClaim, resticRepositoryURL string) error {
	job, err := o.createRestorerJob(ctx, oldPvc, newPvc, resticRepositoryURL)
	if err != nil {
		return err
	}

	return waitForJob(ctx, o.CRClient, job)
}

func (o *Options) createRestorerJob(ctx context.Context,
	oldPvc, newPvc *corev1.PersistentVolumeClaim,
	resticRepositoryURL string) (*batchv1.Job, error) {
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
												Values:   []string{o.TargetNode},
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
							Image:           o.ResticImage,
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
									Value: o.ResticPassword,
								},
							},
							Resources: o.forgeContainerResources(),
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

	if err := o.CRClient.Create(ctx, &job); err != nil {
		return nil, err
	}
	return &job, nil
}
