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

type ServicesReflector struct {
	ri.APIReflector
}

func (r *ServicesReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		IsAllowed:  r.isAllowed,
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

func (r *ServicesReflector) HandleEvent(e interface{}) {
	var err error

	event := e.(watch.Event)
	svc, ok := event.Object.(*corev1.Service)
	if !ok {
		klog.Error("REFLECTION: cannot cast object to service")
		return
	}
	klog.V(3).Infof("REFLECTION: received %v for service %v/%v", event.Type, svc.Namespace, svc.Name)

	switch event.Type {
	case watch.Added:
		_, err := r.GetForeignClient().CoreV1().Services(svc.Namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
		if kerrors.IsAlreadyExists(err) {
			klog.V(3).Infof("REFLECTION: The remote service %v/%v has not been created: %v", svc.Namespace, svc.Name, err)
		}
		if err != nil && !kerrors.IsAlreadyExists(err) {
			klog.Errorf("REFLECTION: Error while creating the remote service %v/%v - ERR: %v", svc.Namespace, svc.Name, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote service %v/%v correctly created", svc.Namespace, svc.Name)
		}

	case watch.Modified:
		if _, err = r.GetForeignClient().CoreV1().Services(svc.Namespace).Update(context.TODO(), svc, metav1.UpdateOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while updating the remote service %v/%v - ERR: %v", svc.Namespace, svc.Name, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote service %v/%v correctly updated", svc.Namespace, svc.Name)
		}

	case watch.Deleted:
		if err := r.GetForeignClient().CoreV1().Services(svc.Namespace).Delete(context.TODO(), svc.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while deleting the remote service %v/%v - ERR: %v", svc.Namespace, svc.Name, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote service %v/%v correctly deleted", svc.Namespace, svc.Name)
		}
	}
}

func (r *ServicesReflector) KeyerFromObj(obj interface{}, remoteNamespace string) string {
	cm, ok := obj.(*corev1.Service)
	if !ok {
		return ""
	}
	return strings.Join([]string{remoteNamespace, cm.Name}, "/")
}

func (r *ServicesReflector) CleanupNamespace(localNamespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(localNamespace, false)
	if err != nil {
		klog.Error(err)
		return
	}

	err = r.ForeignInformer(foreignNamespace).GetStore().Resync()
	if err != nil {
		klog.Errorf("error while resyncing services foreign cache - ERR: %v", err)
		return
	}

	objects := r.ForeignInformer(foreignNamespace).GetStore().List()
	for _, obj := range objects {
		cm := obj.(*corev1.Service)
		if err := r.GetForeignClient().CoreV1().Services(foreignNamespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("error while deleting service %v/%v - ERR: %v", cm.Name, cm.Namespace, err)
		}
	}
}

func (r *ServicesReflector) PreAdd(obj interface{}) interface{} {
	svcLocal := obj.(*corev1.Service)
	klog.V(3).Infof("PreAdd routine started for service %v/%v", svcLocal.Namespace, svcLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(svcLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	svcRemote := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcLocal.Name,
			Namespace: nattedNs,
			Labels:    make(map[string]string),
		},
		Spec: corev1.ServiceSpec{
			Ports:    svcLocal.Spec.Ports,
			Selector: svcLocal.Spec.Selector,
			Type:     svcLocal.Spec.Type,
		},
	}
	for k, v := range svcLocal.Labels {
		svcRemote.Labels[k] = v
	}
	svcRemote.Labels[apimgmt.LiqoLabelKey] = apimgmt.LiqoLabelValue

	klog.V(3).Infof("PreAdd routine completed for service %v/%v", svcLocal.Namespace, svcLocal.Name)
	return svcRemote
}

func (r *ServicesReflector) PreUpdate(newObj interface{}, _ interface{}) interface{} {
	newSvc := newObj.(*corev1.Service).DeepCopy()

	nattedNs, err := r.NattingTable().NatNamespace(newSvc.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}

	key := r.Keyer(nattedNs, newSvc.Name)
	oldRemoteObj, exists, err := r.ForeignInformer(nattedNs).GetStore().GetByKey(key)
	if err != nil {
		klog.Error(err)
		return nil
	}
	if !exists {
		err = r.ForeignInformer(nattedNs).GetStore().Resync()
		if err != nil {
			klog.Errorf("error while resyncing services foreign cache - ERR: %v", err)
			return nil
		}
		oldRemoteObj, exists, err = r.ForeignInformer(nattedNs).GetStore().GetByKey(key)
		if err != nil {
			klog.Errorf("error while retrieving service from foreign cache - ERR: %v", err)
			return nil
		}
		if !exists {
			klog.V(3).Infof("service %v not found after cache resync", key)
			return nil
		}
	}
	RemoteSvc := oldRemoteObj.(*corev1.Service).DeepCopy()

	if RemoteSvc.Labels == nil {
		RemoteSvc.Labels = make(map[string]string)
	}
	for k, v := range newSvc.Labels {
		RemoteSvc.Labels[k] = v
	}
	RemoteSvc.Labels[apimgmt.LiqoLabelKey] = apimgmt.LiqoLabelValue

	if RemoteSvc.Annotations == nil {
		RemoteSvc.Annotations = make(map[string]string)
	}
	for k, v := range newSvc.Annotations {
		RemoteSvc.Annotations[k] = v
	}

	RemoteSvc.Spec.Ports = newSvc.Spec.Ports
	RemoteSvc.Spec.Selector = newSvc.Spec.Selector
	RemoteSvc.Spec.Type = newSvc.Spec.Type

	return RemoteSvc
}

func (r *ServicesReflector) PreDelete(obj interface{}) interface{} {
	svcLocal := obj.(*corev1.Service).DeepCopy()
	klog.V(3).Infof("PreDelete routine started for service %v/%v", svcLocal.Namespace, svcLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(svcLocal.Namespace, false)
	if err != nil {
		klog.Error(err)
		return nil
	}
	svcLocal.Namespace = nattedNs

	klog.V(3).Infof("PreDelete routine completed for service %v/%v", svcLocal.Namespace, svcLocal.Name)
	return svcLocal
}

func (r *ServicesReflector) isAllowed(obj interface{}) bool {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		klog.Error("cannot convert obj to service")
		return false
	}
	key := r.Keyer(svc.Namespace, svc.Name)
	_, ok = blacklist[apimgmt.Services][key]
	if ok {
		klog.V(4).Infof("service %v blacklisted", key)
	}
	return !ok
}

func addServicesIndexers() cache.Indexers {
	i := cache.Indexers{}
	i["services"] = func(obj interface{}) ([]string, error) {
		svc, ok := obj.(*corev1.Service)
		if !ok {
			return []string{}, errors.New("cannot convert obj to service")
		}
		return []string{
			strings.Join([]string{svc.Namespace, svc.Name}, "/"),
		}, nil
	}
	return i
}
