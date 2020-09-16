package outgoing

import (
	"context"
	"errors"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strings"
)

type ConfigmapsReflector struct {
	ri.APIReflector
}

func (r *ConfigmapsReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

func (r *ConfigmapsReflector) HandleEvent(e interface{}) {
	var err error

	event := e.(watch.Event)
	cm, ok := event.Object.(*corev1.ConfigMap)
	if !ok {
		klog.Error("OUTGOING REFLECTION: cannot cast object to configMap")
		return
	}
	klog.V(3).Infof("OUTGOING REFLECTION: received %v for configmap %v/%v", event.Type, cm.Namespace, cm.Name)

	switch event.Type {
	case watch.Added:
		_, err := r.GetForeignClient().CoreV1().ConfigMaps(cm.Namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
		if kerrors.IsAlreadyExists(err) {
			klog.V(3).Infof("OUTGOING REFLECTION: The remote configmap %v/%v has not been created: %v", cm.Namespace, cm.Name, err)
			break
		}

		if err != nil && !kerrors.IsAlreadyExists(err) {
			klog.Errorf("OUTGOING REFLECTION: Error while updating the remote configmap %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		} else {
			klog.V(3).Infof("OUTGOING REFLECTION: remote configMap %v/%v correctly created", cm.Namespace, cm.Name)
		}

	case watch.Modified:
		if _, err = r.GetForeignClient().CoreV1().ConfigMaps(cm.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("OUTGOING REFLECTION: Error while updating the remote configmap %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		} else {
			klog.V(3).Infof("OUTGOING REFLECTION: remote configMap %v/%v correctly updated", cm.Namespace, cm.Name)
		}

	case watch.Deleted:
		if err := r.GetForeignClient().CoreV1().ConfigMaps(cm.Namespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("OUTGOING REFLECTION: Error while deleting the remote configmap %v/%v - ERR: %v", cm.Namespace, cm.Name, err)
		} else {
			klog.V(3).Infof("OUTGOING REFLECTION: remote configMap %v/%v correctly deleted", cm.Namespace, cm.Name)
		}
	}
}

func (r *ConfigmapsReflector) PreAdd(obj interface{}) interface{} {
	cmLocal := obj.(*corev1.ConfigMap)
	klog.V(3).Infof("PreAdd routine started for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(cmLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	cmRemote := &corev1.ConfigMap{
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
	cmRemote.Labels[apimgmt.LiqoLabelKey] = apimgmt.LiqoLabelValue

	klog.V(3).Infof("PreAdd routine completed for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)
	return cmRemote
}

func (r *ConfigmapsReflector) PreUpdate(newObj, _ interface{}) interface{} {
	newCm := newObj.(*corev1.ConfigMap).DeepCopy()

	klog.V(3).Infof("PreUpdate routine started for configmap %v/%v", newCm.Namespace, newCm.Name)

	nattedNs, err := r.NattingTable().NatNamespace(newCm.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	name := r.KeyerFromObj(newObj, nattedNs)
	oldRemoteObj, exists, err := r.ForeignInformer(nattedNs).GetStore().GetByKey(name)
	if err != nil {
		klog.Error(err)
		return nil
	}
	if !exists {
		err = r.ForeignInformer(nattedNs).GetStore().Resync()
		if err != nil {
			klog.Errorf("error while resyncing pods foreign cache - ERR: %v", err)
			return nil
		}
		oldRemoteObj, exists, err = r.ForeignInformer(nattedNs).GetStore().GetByKey(r.Keyer(nattedNs, name))
		if err != nil {
			klog.Errorf("error while retrieving pod from foreign cache - ERR: %v", err)
			return nil
		}
		if !exists {
			klog.V(3).Infof("pod %v/%v not found after cache resync", nattedNs, name)
			return nil
		}
	}
	oldRemoteCm := oldRemoteObj.(*corev1.ConfigMap)

	newCm.SetNamespace(nattedNs)
	newCm.SetResourceVersion(oldRemoteCm.ResourceVersion)
	newCm.SetUID(oldRemoteCm.UID)
	if newCm.Labels == nil {
		newCm.Labels = make(map[string]string)
	}
	for k, v := range oldRemoteCm.Labels {
		newCm.Labels[k] = v
	}
	newCm.Labels[apimgmt.LiqoLabelKey] = apimgmt.LiqoLabelValue

	klog.V(3).Infof("PreUpdate routine completed for configmap %v/%v", newCm.Namespace, newCm.Name)
	return newCm
}

func (r *ConfigmapsReflector) PreDelete(obj interface{}) interface{} {
	cmLocal := obj.(*corev1.ConfigMap)
	klog.V(3).Infof("PreDelete routine started for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(cmLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}
	cmLocal.Namespace = nattedNs

	klog.V(3).Infof("PreDelete routine completed for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)
	return cmLocal
}

func (r *ConfigmapsReflector) KeyerFromObj(obj interface{}, remoteNamespace string) string {
	cm, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return ""
	}
	return strings.Join([]string{remoteNamespace, cm.Name}, "/")
}

func (r *ConfigmapsReflector) CleanupNamespace(localNamespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(localNamespace, false)
	if err != nil {
		klog.Error(err)
		return
	}

	objects := r.ForeignInformer(foreignNamespace).GetStore().List()
	for _, obj := range objects {
		cm := obj.(*corev1.ConfigMap)
		if err := r.GetForeignClient().CoreV1().ConfigMaps(foreignNamespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("error while deleting configmap %v/%v - ERR: %v", cm.Name, cm.Namespace, err)
		}
	}
}

func addConfigmapsIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["configmaps"] = func(obj interface{}) ([]string, error) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return []string{}, errors.New("cannot convert obj to configmap")
		}
		return []string{
			strings.Join([]string{cm.Namespace, cm.Name}, "/"),
		}, nil
	}
	return i
}
