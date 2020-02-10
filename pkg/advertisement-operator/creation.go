package advertisement_operator

import (
	"context"
	"io/ioutil"

	"github.com/go-logr/logr"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/api/v1beta1"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// create a k8s resource of a certain kind from a yaml file
// it is equivalent to "kubectl apply -f file"
func CreateFromYaml(c client.Client, ctx context.Context, log logr.Logger, filename string, kind string) error {

	text, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Error(err, "unable to read file")
		return err
	}

	switch kind {
	case "Pod":
		var pod v1.Pod
		err = yaml.Unmarshal(text, &pod)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = CreateOrUpdate(c, ctx, log, pod)
	case "Deployment":
		var deploy appsv1.Deployment
		err = yaml.Unmarshal(text, &deploy)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = CreateOrUpdate(c, ctx, log, deploy)
	case "ConfigMap":
		var cm v1.ConfigMap
		err = yaml.Unmarshal(text, &cm)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = CreateOrUpdate(c, ctx, log, cm)
	case "ServiceAccount":
		var sa v1.ServiceAccount
		err = yaml.Unmarshal(text, &sa)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = CreateOrUpdate(c, ctx, log, sa)
	case "ClusterRoleBinding":
		var crb rbacv1.ClusterRoleBinding
		err = yaml.Unmarshal(text, &crb)
		if err != nil {
			log.Error(err, "unable to unmarshal yaml file")
			return err
		}
		err = CreateOrUpdate(c, ctx, log, crb)
	default:
		log.Error(err, "invalid kind")
		return err
	}

	return nil
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
				log.Error(err, "unable to create pod")
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update pod")
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
				log.Error(err, "unable to create deployment")
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update deployment")
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
				log.Error(err, "unable to create configMap")
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update configMap")
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
				log.Error(err, "unable to create serviceAccount")
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update serviceAccount")
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
				log.Error(err, "unable to create clusterRoleBinding")
				return err
			}
		} else {
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update clusterRoleBinding")
				return err
			}
		}
	case protocolv1beta1.Advertisement:
		var adv protocolv1beta1.Advertisement
		err := c.Get(ctx, types.NamespacedName{
			Namespace: obj.Namespace,
			Name:      obj.Name,
		}, &adv)
		if err != nil {
			err = c.Create(ctx, &obj, &client.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				log.Error(err, "unable to create advertisement")
				return err
			}
		} else {
			obj.SetResourceVersion(adv.ResourceVersion)
			err = c.Update(ctx, &obj, &client.UpdateOptions{})
			if err != nil {
				log.Error(err, "unable to update advertisement")
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
