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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/pkg/utils/resource"
)

func (o *Options) ensureResticRepository(ctx context.Context, targetPvc *corev1.PersistentVolumeClaim) error {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resticRegistry,
			Namespace: liqoStorageNamespace,
		},
	}
	_, err := resource.CreateOrUpdate(ctx, o.CRClient, &svc, func() error {
		svc.Spec = corev1.ServiceSpec{
			Selector: map[string]string{
				"app": resticRegistry,
			},
			Ports: []corev1.ServicePort{
				{
					Port:       resticPort,
					TargetPort: intstr.FromInt(resticPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}

	statefulSet := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resticRegistry,
			Namespace: liqoStorageNamespace,
		},
	}
	_, err = resource.CreateOrUpdate(ctx, o.CRClient, &statefulSet, func() error {
		statefulSet.Spec = appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": resticRegistry,
				},
			},
			ServiceName: resticRegistry,
			Replicas:    pointer.Int32Ptr(1),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": resticRegistry,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  resticRegistry,
							Image: o.ResticServerImage,
							Env: []corev1.EnvVar{
								{
									Name:  "DISABLE_AUTHENTICATION",
									Value: "1",
								},
								{
									Name:  "OPTIONS",
									Value: "--no-auth",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: resticPort,
								},
							},
							Resources: o.forgeContainerResources(),
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "restic-registry-data",
									MountPath: "/data",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "restic-registry-data",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "restic-registry-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: targetPvc.Spec.Resources.Requests[corev1.ResourceStorage],
							},
						},
					},
				},
			},
		}
		return nil
	})
	return err
}

func deleteResticRepository(ctx context.Context, cl client.Client) error {
	if err := cl.Delete(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resticRegistry,
			Namespace: liqoStorageNamespace,
		},
	}); client.IgnoreNotFound(err) != nil {
		return err
	}

	if err := scaleResticRepository(ctx, cl); err != nil {
		return err
	}

	if err := cl.Delete(ctx, &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resticRegistry,
			Namespace: liqoStorageNamespace,
		},
	}); client.IgnoreNotFound(err) != nil {
		return err
	}

	return client.IgnoreNotFound(cl.Delete(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "restic-registry-data-restic-registry-0",
			Namespace: liqoStorageNamespace,
		},
	}))
}

func scaleResticRepository(ctx context.Context, cl client.Client) error {
	statefulSet := appsv1.StatefulSet{}
	if err := cl.Get(ctx, client.ObjectKey{
		Name:      resticRegistry,
		Namespace: liqoStorageNamespace,
	}, &statefulSet); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	statefulSet.Spec.Replicas = pointer.Int32Ptr(0)
	if err := cl.Update(ctx, &statefulSet); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	ticker := time.NewTicker(time.Second * 3)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for restic repository to scale down")
		case <-ticker.C:
			if err := cl.Get(timeoutCtx, client.ObjectKey{
				Name:      resticRegistry,
				Namespace: liqoStorageNamespace,
			}, &statefulSet); err != nil {
				return err
			}
			if statefulSet.Status.ReadyReplicas == 0 {
				return nil
			}
		}
	}
}

func waitForResticRepository(ctx context.Context, cl client.Client) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute*5)
	defer cancel()
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	var statefulSet appsv1.StatefulSet
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := cl.Get(ctx, client.ObjectKey{
				Name:      resticRegistry,
				Namespace: liqoStorageNamespace,
			}, &statefulSet); err != nil {
				return err
			}

			if statefulSet.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}
