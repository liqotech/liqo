package advertisementOperator

import (
	"context"
	"errors"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// create deployment for a virtual-kubelet
func CreateVkDeployment(adv *advtypes.Advertisement, vkName, vkNamespace, vkImage, initVKImage, nodeName, homeClusterId string) *appsv1.Deployment {

	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		"--foreign-cluster-id",
		adv.Spec.ClusterId,
		"--provider",
		"kubernetes",
		"--nodename",
		nodeName,
		"--kubelet-namespace",
		vkNamespace,
		"--foreign-kubeconfig",
		"/app/kubeconfig/remote",
		"--home-cluster-id",
		homeClusterId,
	}

	volumes := []v1.Volume{
		{
			Name: "remote-kubeconfig",
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: adv.Spec.KubeConfigRef.Name,
				},
			},
		},
		{
			Name: "virtual-kubelet-crt",
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		},
	}

	volumeMounts := []v1.VolumeMount{
		{
			Name:      "remote-kubeconfig",
			MountPath: "/app/kubeconfig/remote",
			SubPath:   "kubeconfig",
		},
		{
			Name:      "virtual-kubelet-crt",
			MountPath: "/etc/virtual-kubelet/certs",
		},
	}

	affinity := v1.Affinity{
		NodeAffinity: &v1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{
					{
						MatchExpressions: []v1.NodeSelectorRequirement{
							{
								Key:      "type",
								Operator: v1.NodeSelectorOpNotIn,
								Values:   []string{"virtual-node"},
							},
						},
					},
				},
			},
		},
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            vkName,
			Namespace:       vkNamespace,
			OwnerReferences: GetOwnerReference(adv),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "virtual-kubelet",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "virtual-kubelet",
						"cluster": adv.Spec.ClusterId,
					},
				},
				Spec: v1.PodSpec{
					Volumes: volumes,
					InitContainers: []v1.Container{
						{
							Name:  "crt-generator",
							Image: initVKImage,
							Command: []string{
								"/usr/bin/local/kubelet-setup.sh",
							},
							Env: []v1.EnvVar{
								{
									Name:      "POD_IP",
									ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP", APIVersion: "v1"}},
								},
								{
									Name:      "POD_NAME",
									ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.name", APIVersion: "v1"}},
								},
								{
									Name:  "NODE_NAME",
									Value: nodeName,
								},
							},
							Args: []string{
								"/etc/virtual-kubelet/certs",
							},
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      "virtual-kubelet-crt",
									MountPath: "/etc/virtual-kubelet/certs",
								},
							},
						},
					},
					Containers: []v1.Container{
						{
							Name:            "virtual-kubelet",
							Image:           vkImage,
							ImagePullPolicy: v1.PullAlways,
							Command:         command,
							Args:            args,
							VolumeMounts:    volumeMounts,
							Env: []v1.EnvVar{
								{
									Name:  "APISERVER_CERT_LOCATION",
									Value: "/etc/virtual-kubelet/certs/server.crt",
								},
								{
									Name:  "APISERVER_KEY_LOCATION",
									Value: "/etc/virtual-kubelet/certs/server-key.pem",
								},
								{
									Name:      "VKUBELET_POD_IP",
									ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "status.podIP", APIVersion: "v1"}},
								},
								{
									Name:  "VKUBELET_TAINT_KEY",
									Value: "virtual-node.liqo.io/not-allowed",
								},
								{
									Name:  "VKUBELET_TAINT_VALUE",
									Value: "true",
								},
								{
									Name:  "VKUBELET_TAINT_EFFECT",
									Value: "NoExecute",
								},
							},
						},
					},
					ServiceAccountName: vkName,
					Affinity:           affinity.DeepCopy(),
				},
			},
		},
	}

	return deploy
}

// create a k8s resource or update it if already exists
// it receives a pointer to the resource
func CreateOrUpdate(c client.Client, ctx context.Context, object interface{}) error {

	switch obj := object.(type) {
	case *v1.Pod:
		var pod v1.Pod
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &pod)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(pod.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	case *appsv1.Deployment:
		var deploy appsv1.Deployment
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &deploy)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(deploy.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}

	case *v1.ConfigMap:
		var cm v1.ConfigMap
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &cm)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(cm.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	case *v1.Secret:
		var sec v1.Secret
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &sec)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(sec.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	case *v1.ServiceAccount:
		var sa v1.ServiceAccount
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &sa)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(sa.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	case *rbacv1.ClusterRoleBinding:
		var crb rbacv1.ClusterRoleBinding
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &crb)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(crb.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	case *advtypes.Advertisement:
		var adv advtypes.Advertisement
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &adv)
		if err != nil {
			err = c.Create(ctx, obj, &client.CreateOptions{})
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			obj.SetResourceVersion(adv.ResourceVersion)
			err = c.Update(ctx, obj, &client.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	default:
		err := errors.New("invalid kind")
		return err
	}

	return nil
}
