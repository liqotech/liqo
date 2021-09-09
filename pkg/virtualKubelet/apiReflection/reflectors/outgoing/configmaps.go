// Copyright 2019-2021 The Liqo Authors
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

package outgoing

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
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
			klog.V(3).Infof("OUTGOING REFLECTION: The remote configmap %v/%v has not been created because already existing", cm.Namespace, cm.Name)
			break
		}
		if err != nil {
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

func (r *ConfigmapsReflector) PreAdd(obj interface{}) (interface{}, watch.EventType) {
	cmLocal := obj.(*corev1.ConfigMap)
	klog.V(3).Infof("PreAdd routine started for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(cmLocal.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Added
	}

	cmRemote := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cmLocal.Name,
			Namespace:   nattedNs,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Data:       cmLocal.Data,
		BinaryData: cmLocal.BinaryData,
	}
	for k, v := range cmLocal.Labels {
		cmRemote.Labels[k] = v
	}
	cmRemote.Labels[forge.LiqoOutgoingKey] = forge.LiqoNodeName()

	klog.V(3).Infof("PreAdd routine completed for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)
	return cmRemote, watch.Added
}

func (r *ConfigmapsReflector) PreUpdate(newObj, _ interface{}) (interface{}, watch.EventType) {
	newHomeCm := newObj.(*corev1.ConfigMap).DeepCopy()

	klog.V(3).Infof("PreUpdate routine started for configmap %v/%v", newHomeCm.Namespace, newHomeCm.Name)

	nattedNs, err := r.NattingTable().NatNamespace(newHomeCm.Namespace)
	if err != nil {
		err = errors.Wrapf(err, "configmap %v/%v", nattedNs, newHomeCm.Name)
		klog.Error(err)
		return nil, watch.Modified
	}

	oldForeignObj, err := r.GetCacheManager().GetForeignNamespacedObject(apimgmt.Configmaps, nattedNs, newHomeCm.Name)
	if err != nil {
		err = errors.Wrapf(err, "configmap %v/%v", nattedNs, newHomeCm.Name)
		klog.Error(err)
		return nil, watch.Modified
	}

	oldRemoteCm := oldForeignObj.(*corev1.ConfigMap)

	newHomeCm.SetNamespace(nattedNs)
	newHomeCm.SetResourceVersion(oldRemoteCm.ResourceVersion)
	newHomeCm.SetUID(oldRemoteCm.UID)
	if newHomeCm.Labels == nil {
		newHomeCm.Labels = make(map[string]string)
	}
	for k, v := range oldRemoteCm.Labels {
		newHomeCm.Labels[k] = v
	}
	newHomeCm.Labels[forge.LiqoOutgoingKey] = forge.LiqoNodeName()

	if newHomeCm.Annotations == nil {
		newHomeCm.Annotations = make(map[string]string)
	}
	for k, v := range oldRemoteCm.Annotations {
		newHomeCm.Annotations[k] = v
	}

	klog.V(3).Infof("PreUpdate routine completed for configmap %v/%v", newHomeCm.Namespace, newHomeCm.Name)
	return newHomeCm, watch.Modified
}

func (r *ConfigmapsReflector) PreDelete(obj interface{}) (interface{}, watch.EventType) {
	cmLocal := obj.(*corev1.ConfigMap).DeepCopy()
	klog.V(3).Infof("PreDelete routine started for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(cmLocal.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Deleted
	}
	cmLocal.Namespace = nattedNs

	klog.V(3).Infof("PreDelete routine completed for configmap %v/%v", cmLocal.Namespace, cmLocal.Name)
	return cmLocal, watch.Deleted
}

func (r *ConfigmapsReflector) CleanupNamespace(localNamespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(localNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ListForeignNamespacedObject(apimgmt.Configmaps, foreignNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting configmap because of- ERR; %v", err)
			return true
		}
	}
	for _, obj := range objects {
		cm := obj.(*corev1.ConfigMap)
		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetForeignClient().CoreV1().ConfigMaps(foreignNamespace).Delete(context.TODO(), cm.Name, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting remote configmap %v/%v", cm.Namespace, cm.Name)
		}
	}
}
