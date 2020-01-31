/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"io/ioutil"

	"github.com/go-logr/logr"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/advertisement-operator/api/v1beta1"
)

// AdvertisementReconciler reconciles a Advertisement object
type AdvertisementReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=protocol.drone.com,resources=advertisements/status,verbs=get;update;patch

func (r *AdvertisementReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("advertisement", req.NamespacedName)

	// get advertisement
	var adv protocolv1beta1.Advertisement
	if err := r.Get(ctx, req.NamespacedName, &adv); err != nil {
		log.Error(err, "unable to fetch Advertisement")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// filter advertisements and create a virtual-kubelet only for the good ones
	adv = checkAdvertisement(adv)
	if adv.Status.Phase != "ACCEPTED" {
		return ctrl.Result{}, errors.NewBadRequest("advertisement ignored")
	}

	// create configuration for virtual-kubelet with data from adv
	vkConfig := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"vkubelet-cfg.json": `
		{
		 "virtual-kubelet": {
		   "remoteKubeconfig" : "/app/kubeconfig/remote",
		   "namespace": "drone-v2",
		   "cpu": "` + adv.Spec.Availability.Cpu.String() + `",
		   "memory": "` + adv.Spec.Availability.Ram.String() + `",
		   "pods": "128"
		 }
		}`},
	}

	//TODO: update if already exist
	if err := r.Create(ctx, &vkConfig, &client.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		log.Error(err, "unable to create configMap")
		return ctrl.Result{}, err
	}

	// create resources necessary to run virtual-kubelet deployment
	//err := CreateFromYaml(r, ctx, log, "./data/vk-config_cm.yaml", "ConfigMap")
	err := CreateFromYaml(r, ctx, log, "./data/foreignKubeconfig_cm.yaml", "ConfigMap")
	if err != nil {
		return ctrl.Result{}, err
	}
	err = CreateFromYaml(r, ctx, log, "./data/vk_sa.yaml", "ServiceAccount")
	if err != nil {
		return ctrl.Result{}, err
	}
	err = CreateFromYaml(r, ctx, log, "./data/vk_crb.yaml", "ClusterRoleBinding")
	if err != nil {
		return ctrl.Result{}, err
	}
	err = CreateFromYaml(r, ctx, log, "./data/vk_deploy.yaml", "Deployment")
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *AdvertisementReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&protocolv1beta1.Advertisement{}).
		Complete(r)
}

// check if the advertisement is interesting and set its status accordingly
func checkAdvertisement(adv protocolv1beta1.Advertisement) protocolv1beta1.Advertisement {
	//TODO: implement logic
	adv.Status.Phase = "ACCEPTED"
	return adv
}

// create a k8s resource of a certain kind from a file
// it equivalent to "kubectl apply -f file"
func CreateFromYaml(r *AdvertisementReconciler, ctx context.Context, log logr.Logger, filename string, kind string) error {

	text, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error(err, "unable to read file")
		return err
	}

	switch kind {
	case "Pod":
		var pod, tmp v1.Pod
		err = yaml.Unmarshal(text, &pod)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}

		err = r.Get(ctx, types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}, &tmp)
		if err != nil {
			err = r.Create(ctx, &pod, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create pod")
				return err
			}
		} else {
			err = r.Update(ctx, &pod, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update pod")
				return err
			}
		}
	case "Deployment":
		var deploy, tmp appsv1.Deployment
		err = yaml.Unmarshal(text, &deploy)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
		}, &tmp)
		if err != nil {
			err = r.Create(ctx, &deploy, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create deployment")
				return err
			}
		} else {
			err = r.Update(ctx, &deploy, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update deployment")
				return err
			}
		}
	case "ConfigMap":
		var cm, tmp v1.ConfigMap
		err = yaml.Unmarshal(text, &cm)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: cm.Namespace,
			Name:      cm.Name,
		}, &tmp)
		if err != nil {
			err = r.Create(ctx, &cm, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create configMap")
				return err
			}
		} else {
			err = r.Update(ctx, &cm, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update deployment")
				return err
			}
		}
	case "ServiceAccount":
		var sa, tmp v1.ServiceAccount
		err = yaml.Unmarshal(text, &sa)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: sa.Namespace,
			Name:      sa.Name,
		}, &tmp)
		if err != nil {
			err = r.Create(ctx, &sa, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create serviceAccount")
				return err
			}
		} else {
			err = r.Update(ctx, &sa, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update serviceAccount")
				return err
			}
		}
	case "ClusterRoleBinding":
		var crb, tmp rbacv1.ClusterRoleBinding
		err = yaml.Unmarshal(text, &crb)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = r.Get(ctx, types.NamespacedName{
			Namespace: crb.Namespace,
			Name:      crb.Name,
		}, &tmp)
		if err != nil {
			err = r.Create(ctx, &crb, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create clusterRoleBinding")
				return err
			}
		} else {
			err = r.Update(ctx, &crb, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update clusterRoleBinding")
				return err
			}
		}
	default:
		log.Error(err, "invalid kind")
		return err
	}

	return nil
}
