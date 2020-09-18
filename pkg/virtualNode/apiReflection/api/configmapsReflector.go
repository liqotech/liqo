package api

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

type ConfigmapsReflector struct {
	GenericAPIReflector
}

func (r *ConfigmapsReflector) SetPreProcessingHandlers() {
	r.PreProcessingHandlers = PreProcessingHandlers{
			addFunc:    r.PreAdd,
			updateFunc: r.PreUpdate,
			deleteFunc: r.PreDelete,
	}
}

func (r *ConfigmapsReflector) HandleEvent(e interface{}) {
	var err error

	event := e.(watch.Event)
	cm, ok := event.Object.(*corev1.ConfigMap)
	if !ok {
		klog.Error("REFLECTION: cannot cast object to configMap")
		return
	}
	klog.V(3).Infof("REFLECTION: received %v for configmap %v", event.Type, cm.Name)

	switch event.Type {
	case watch.Added:
		if _, err := r.ForeignClient.CoreV1().ConfigMaps(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while creating the remote configmap %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		}
		klog.V(3).Infof("REFLECTION: remote configMap %v-%v correctly created",cm.Namespace, cm.Name)

	case watch.Modified:
		if _, err = r.ForeignClient.CoreV1().ConfigMaps(cm.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while updating the remote configmap %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		}
		klog.V(3).Infof("REFLECTION: remote configMap %v-%v correctly updated",cm.Namespace, cm.Name)

	case watch.Deleted:
		if err := r.ForeignClient.CoreV1().ConfigMaps(cm.Namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while deleting the remote configmap %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		}
		klog.V(3).Infof("REFLECTION: remote configMap %v-%v correctly deleted",cm.Namespace, cm.Name)
	}
}

func (r *ConfigmapsReflector) PreAdd(obj interface{}) interface{} {
	cmLocal := obj.(corev1.ConfigMap)
	nattedNs, err := r.NamespaceNatting.NatNamespace(cmLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	cmRemote := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmLocal.Name,
			Namespace: nattedNs,
			Labels:    make(map[string]string),
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

func (r *ConfigmapsReflector) PreUpdate(newObj, _ interface{}) interface{} {
	cmNewLocal := newObj.(corev1.ConfigMap)

	nattedNs, err := r.NamespaceNatting.NatNamespace(cmNewLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	cmOldRemote, err := r.ForeignClient.CoreV1().ConfigMaps(nattedNs).Get(context.TODO(), cmNewLocal.Name, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil
	}
	cmNewLocal.SetNamespace(nattedNs)
	cmNewLocal.SetResourceVersion(cmOldRemote.ResourceVersion)
	cmNewLocal.SetUID(cmOldRemote.UID)

	return newObj
}

func (r *ConfigmapsReflector) PreDelete(obj interface{}) interface{} {
	cmLocal := obj.(corev1.ConfigMap)
	nattedNs, err := r.NamespaceNatting.NatNamespace(cmLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}
	cmLocal.Namespace = nattedNs

	return cmLocal
}
