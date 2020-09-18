package apis

import (
	"context"
	"errors"
	"github.com/liqotech/liqo/pkg/virtualNode/apiReflection/reflectionController"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

type ConfigmapsReflector struct {
	reflectionController.GenericAPIReflector
}

func (r *ConfigmapsReflector) HandleEvent(e interface{}) error {
	var err error

	event := e.(watch.Event)
	cm, ok := event.Object.(*corev1.ConfigMap)
	if !ok {
		return errors.New("cannot cast object to configMap")
	}
	klog.V(3).Infof("received %v on configmap %v", event.Type, cm.Name)

	nattedNS, err := p.NatNamespace(cm.Namespace, false)
	if err != nil {
		return err
	}

	switch event.Type {
	case watch.Added:
		_, err := r.foreignClient.CoreV1().ConfigMaps(nattedNS).Get(context.TODO(), cm.Name, metav1.GetOptions{})
		if err != nil {
			klog.Info("remote cm " + cm.Name + " doesn't exist: creating it")

			if err = r.createConfigMap(cm, nattedNS); err != nil {
				klog.Errorf("unable to create configMap %v - ERR: %v", cm.Name, err)
			} else {
				klog.V(3).Infof("configMap %v correctly created", cm.Name)
			}
		}

	case watch.Modified:
		if err = r.updateConfigMap(cm, nattedNS); err != nil {
			klog.Errorf("unable to update configMap %v - ERR: %v", cm.Name, err)
		} else {
			klog.V(3).Infof("configMap %v correctly updated", cm.Name)
		}

	case watch.Deleted:
		if err = r.deleteConfigMap(cm, nattedNS); err != nil {
			klog.Errorf("unable to delete configMap %v - ERR: %v", cm.Name, err)
		} else {
			klog.V(3).Infof("ConfigMap %v correctly deleted", cm.Name)
		}
	}
	return nil
}

func (r *ConfigmapsReflector)PreProcessingAdd(obj interface{}) interface{} {
	cmLocal := obj.(corev1.ConfigMap)
	cmRemote := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cmLocal.Name,
			Namespace:   "", // TODO natting
			Labels:      make(map[string]string),
		},
		Data:       cmLocal.Data,
		BinaryData: cmLocal.BinaryData,
	}
	for k, v := range cmLocal.Labels {
		cmRemote.Labels[k] = v
	}
	cmRemote.Labels["liqo/reflection"] = "reflected"

	return cmRemote
}

func (r *ConfigmapsReflector)PreProcessingUpdate(newObj, oldObj interface{}) interface{} {
	return newObj
}

func (r *ConfigmapsReflector)PreProcessingDelete(obj interface{}) interface{} {
	return obj
}

func (r *ConfigmapsReflector) createConfigMap(cm *corev1.ConfigMap, namespace string) error {
	_, err := r.foreignClient.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	return err
}

func (r *ConfigmapsReflector) updateConfigMap(cm *corev1.ConfigMap, namespace string) error {
	cmOld, err := r.foreignClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), cm.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	cm2 := cm.DeepCopy()
	cm2.SetNamespace(namespace)
	cm2.SetResourceVersion(cmOld.ResourceVersion)
	cm2.SetUID(cmOld.UID)
	_, err = r.foreignClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm2, metav1.UpdateOptions{})

	return err
}

func (r *ConfigmapsReflector) deleteConfigMap(cm *corev1.ConfigMap, namespace string) error {
	cm.Namespace = namespace
	err := r.foreignClient.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})

	return err
}
