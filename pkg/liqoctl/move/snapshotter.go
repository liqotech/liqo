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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *Options) takeSnapshot(ctx context.Context, pvc *corev1.PersistentVolumeClaim, resticRepositoryURL string) error {
	job, err := o.createSnapshotterJob(ctx, pvc, resticRepositoryURL)
	if err != nil {
		return err
	}

	return waitForJob(ctx, o.CRClient, job)
}

func (o *Options) createSnapshotterJob(ctx context.Context, pvc *corev1.PersistentVolumeClaim,
	resticRepositoryURL string) (*batchv1.Job, error) {
	var pv corev1.PersistentVolume
	if err := o.CRClient.Get(ctx, client.ObjectKey{Name: pvc.Spec.VolumeName}, &pv); err != nil {
		return nil, err
	}

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "liqo-snapshotter-",
			Namespace:    pvc.GetNamespace(),
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: pointer.Int32Ptr(10),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:            "restic-init",
							Image:           o.ResticImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"-r",
								fmt.Sprintf("%s%s", resticRepositoryURL, pvc.GetUID()),
								"init",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "RESTIC_PASSWORD",
									Value: o.ResticPassword,
								},
							},
							Resources: o.forgeContainerResources(),
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "restic",
							Image:           o.ResticImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"-r",
								fmt.Sprintf("%s%s", resticRepositoryURL, pvc.GetUID()),
								"backup", ".",
								"--host", "liqo",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "RESTIC_PASSWORD",
									Value: o.ResticPassword,
								},
							},
							Resources:  o.forgeContainerResources(),
							WorkingDir: "/backup",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "backup",
									MountPath: "/backup",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Volumes: []corev1.Volume{
						{
							Name: "backup",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.GetName(),
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

func waitForJob(ctx context.Context, cl client.Client, job *batchv1.Job) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := cl.Get(ctx, client.ObjectKey{
				Name:      job.GetName(),
				Namespace: job.GetNamespace(),
			}, job); err != nil {
				continue
			}

			if job.Status.Succeeded > 0 {
				return nil
			}
		}
	}
}
