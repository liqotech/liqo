package advertisement_operator

import (
	"context"
	"github.com/go-logr/logr"
	"io/ioutil"

	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// create a k8s resource of a certain kind from a yaml file
// it is equivalent to "kubectl apply -f *.yaml"
func CreateFromYaml(c client.Client, ctx context.Context, log logr.Logger, filename string, kind string) (interface{}, error) {

	text, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error(err, "unable to read file "+filename)
		return nil, err
	}

	switch kind {
	case "Pod":
		var pod v1.Pod
		err = yaml.Unmarshal(text, &pod)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file "+filename)
			return nil, err
		}
		return pod, nil
	case "Deployment":
		var deploy appsv1.Deployment
		err = yaml.Unmarshal(text, &deploy)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file "+filename)
			return nil, err
		}
		return deploy, nil
	case "ConfigMap":
		var cm v1.ConfigMap
		err = yaml.Unmarshal(text, &cm)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file "+filename)
			return nil, err
		}
		return cm, nil
	case "ServiceAccount":
		var sa v1.ServiceAccount
		err = yaml.Unmarshal(text, &sa)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file "+filename)
			return nil, err
		}
		return sa, nil
	case "ClusterRoleBinding":
		var crb rbacv1.ClusterRoleBinding
		err = yaml.Unmarshal(text, &crb)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file "+filename)
			return nil, err
		}
		return crb, nil
	default:
		log.Error(err, "invalid kind")
		return nil, err
	}
}

// create deployment for a virtual-kubelet
func CreateVkDeployment(adv *protocolv1.Advertisement, nameSA string) appsv1.Deployment {

	command := []string{
		"/usr/bin/virtual-kubelet",
	}

	args := []string{
		"--cluster-id",
		adv.Spec.ClusterId,
		"--provider",
		"kubernetes",
		"--provider-config",
		"/app/kubeconfig/remote",
		"--disable-taint",
		"--nodename",
		"vk-" + adv.Spec.ClusterId,
	}

	volumes := []v1.Volume{
		{
			Name: "provider-config",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{Name: "vk-config-" + adv.Spec.ClusterId},
				},
			},
		},
		{
			Name: "remote-kubeconfig",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{Name: "foreign-kubeconfig-" + adv.Spec.ClusterId},
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
			Name:      "provider-config",
			MountPath: "/app/config/vkubelet-cfg.json",
			SubPath:   "vkubelet-cfg.json",
		},
		{
			Name:      "remote-kubeconfig",
			MountPath: "/app/kubeconfig/remote",
			SubPath:   "remote",
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

	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "vkubelet-" + adv.Spec.ClusterId,
			Namespace:       "default",
			OwnerReferences: GetOwnerReference(*adv),
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
						"app": "virtual-kubelet",
						"cluster": adv.Spec.ClusterId,
					},
				},
				Spec: v1.PodSpec{
					Volumes: volumes,
					InitContainers: []v1.Container{
						{
							Name:  "crt-generator",
							Image: "dronev2/init-vkubelet",
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
							Image:           "dronev2/virtual-kubelet",
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
							},
						},
					},
					ServiceAccountName: nameSA,
					Affinity: affinity.DeepCopy(),
				},
			},
		},
	}

	return deploy
}

// create a k8s resource or update it if already exists
func CreateOrUpdate(c client.Client, ctx context.Context, log logr.Logger, object interface{}) error {

	switch obj := object.(type) {
	case v1.Pod:
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, new(v1.Pod))
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create pod "+obj.Name)
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update pod "+obj.Name)
				return err
			}
		}
	case appsv1.Deployment:
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, new(appsv1.Deployment))
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create deployment "+obj.Name)
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update deployment "+obj.Name)
				return err
			}
		}
	case v1.ConfigMap:
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, new(v1.ConfigMap))
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create configMap "+obj.Name)
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update configMap "+obj.Name)
				return err
			}
		}
	case v1.ServiceAccount:
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, new(v1.ServiceAccount))
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create serviceAccount "+obj.Name)
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update serviceAccount "+obj.Name)
				return err
			}
		}
	case rbacv1.ClusterRoleBinding:
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, new(rbacv1.ClusterRoleBinding))
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create clusterRoleBinding "+obj.Name)
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update clusterRoleBinding "+obj.Name)
				return err
			}
		}
	case protocolv1.Advertisement:
		var adv protocolv1.Advertisement
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &adv)
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create advertisement "+obj.Name)
				return err
			}
		} else {
			obj.SetResourceVersion(adv.ResourceVersion)
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update advertisement "+obj.Name)
				return err
			}
		}
	default:
		var err error
		log.Error(err, "invalid kind")
		return err
	}

	return nil
}

func CreateFromFile(c client.Client, ctx context.Context, log logr.Logger, filename string) error {
	text, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error(err, "unable to read file"+filename)
		return err
	}

	remoteKubeConfig := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foreign-kubeconfig",
			Namespace: "default",
		},
		Data: map[string]string{
			"remote": string(text),
		},
	}
	err = CreateOrUpdate(c, ctx, log, remoteKubeConfig)
	if err != nil {
		return err
	}

	return nil
}
