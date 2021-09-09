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

type SecretsReflector struct {
	ri.APIReflector
}

func (r *SecretsReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		IsAllowed:  r.isAllowed,
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

func (r *SecretsReflector) HandleEvent(e interface{}) {
	event := e.(watch.Event)
	secret, ok := event.Object.(*corev1.Secret)
	if !ok {
		klog.Error("REFLECTION: cannot cast object to Secret")
		return
	}
	klog.V(3).Infof("REFLECTION: received %v for Secret %v/%v", event.Type, secret.Namespace, secret.Name)

	switch event.Type {
	case watch.Added:
		_, err := r.GetForeignClient().CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if kerrors.IsAlreadyExists(err) {
			klog.V(4).Infof("REFLECTION: The remote Secret %v/%v has not been created because already existing", secret.Namespace, secret.Name)
			break
		}
		if err != nil && !kerrors.IsAlreadyExists(err) {
			klog.Errorf("REFLECTION: Error while updating the remote Secret %v/%v - ERR: %v", secret.Namespace, secret.Name, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote Secret %v/%v correctly created", secret.Namespace, secret.Name)
		}

	case watch.Modified:
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			_, newErr := r.GetForeignClient().CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
			return newErr
		}); err != nil {
			klog.Errorf("REFLECTION: Error while updating the remote Secret %v/%v - ERR: %v", secret.Namespace, secret.Name, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote Secret %v/%v correctly updated", secret.Namespace, secret.Name)
		}

	case watch.Deleted:
		if err := r.GetForeignClient().CoreV1().Secrets(secret.Namespace).Delete(context.TODO(), secret.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("REFLECTION: Error while deleting the remote Secret %v/%v - ERR: %v", secret.Namespace, secret.Name, err)
		} else {
			klog.V(3).Infof("REFLECTION: remote Secret %v/%v correctly deleted", secret.Namespace, secret.Name)
		}
	}
}

func (r *SecretsReflector) CleanupNamespace(localNamespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(localNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ListForeignNamespacedObject(apimgmt.Secrets, foreignNamespace)
	if err != nil {
		klog.Error(err)
		return
	}

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting secret because of- ERR; %v", err)
			return true
		}
	}
	for _, obj := range objects {
		sec := obj.(*corev1.Secret)
		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetForeignClient().CoreV1().Secrets(foreignNamespace).Delete(context.TODO(), sec.Name, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting secret %v/%v", sec.Namespace, sec.Name)
		}
	}
}

func (r *SecretsReflector) PreAdd(obj interface{}) (interface{}, watch.EventType) {
	secretLocal := obj.(*corev1.Secret).DeepCopy()
	klog.V(3).Infof("PreAdd routine started for Secret %v/%v", secretLocal.Namespace, secretLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(secretLocal.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Added
	}

	secretRemote := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretLocal.Name,
			Namespace:   nattedNs,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Data:       secretLocal.Data,
		StringData: secretLocal.StringData,
		Type:       secretLocal.Type,
	}

	for k, v := range secretLocal.Annotations {
		secretRemote.Annotations[k] = v
	}
	for k, v := range secretLocal.Labels {
		secretRemote.Labels[k] = v
	}
	secretRemote.Labels[forge.LiqoOutgoingKey] = forge.LiqoNodeName()

	// if the secret was generated by a ServiceAccount we can reflect it because
	// creation of secret with ServiceAccountToken type is not allowed by the API Server
	//
	// so we change the type of the secret and we label it with the name of the
	// ServiceAccount that originated it to be easy retrieved when we need it
	if secretRemote.Type == corev1.SecretTypeServiceAccountToken {
		secretRemote.Type = corev1.SecretTypeOpaque
		secretRemote.Labels["kubernetes.io/service-account.name"] = secretLocal.Annotations["kubernetes.io/service-account.name"]
		delete(secretRemote.Annotations, "kubernetes.io/service-account.name")
		delete(secretRemote.Annotations, "kubernetes.io/service-account.uid")
	}

	klog.V(3).Infof("PreAdd routine completed for secret %v/%v", secretLocal.Namespace, secretLocal.Name)
	return secretRemote, watch.Added
}

func (r *SecretsReflector) PreUpdate(newObj interface{}, _ interface{}) (interface{}, watch.EventType) {
	newSecret := newObj.(*corev1.Secret).DeepCopy()
	secretName := newSecret.Name

	nattedNs, err := r.NattingTable().NatNamespace(newSecret.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Modified
	}

	oldRemoteObj, err := r.GetCacheManager().GetForeignNamespacedObject(apimgmt.Secrets, nattedNs, secretName)
	if err != nil {
		err = errors.Wrapf(err, "secret %v%v", nattedNs, secretName)
		klog.Error(err)
		return nil, watch.Modified
	}
	oldRemoteSec := oldRemoteObj.(*corev1.Secret)

	newSecret.SetNamespace(nattedNs)
	newSecret.SetResourceVersion(oldRemoteSec.ResourceVersion)
	newSecret.SetUID(oldRemoteSec.UID)

	if newSecret.Labels == nil {
		newSecret.Labels = make(map[string]string)
	}
	for k, v := range oldRemoteSec.Labels {
		newSecret.Labels[k] = v
	}
	newSecret.Labels[forge.LiqoOutgoingKey] = forge.LiqoNodeName()

	if newSecret.Annotations == nil {
		newSecret.Annotations = make(map[string]string)
	}
	for k, v := range oldRemoteSec.Annotations {
		newSecret.Annotations[k] = v
	}

	if newSecret.Type == corev1.SecretTypeServiceAccountToken {
		newSecret.Type = corev1.SecretTypeOpaque
		newSecret.Labels["kubernetes.io/service-account.name"] = newSecret.Annotations["kubernetes.io/service-account.name"]
		delete(newSecret.Annotations, "kubernetes.io/service-account.name")
		delete(newSecret.Annotations, "kubernetes.io/service-account.uid")
	}

	klog.V(3).Infof("PreUpdate routine completed for secret %v/%v", newSecret.Namespace, newSecret.Name)

	return newSecret, watch.Modified
}

func (r *SecretsReflector) PreDelete(obj interface{}) (interface{}, watch.EventType) {
	secretLocal := obj.(*corev1.Secret).DeepCopy()

	klog.V(3).Infof("PreDelete routine started for secret %v/%v", secretLocal.Namespace, secretLocal.Name)

	nattedNs, err := r.NattingTable().NatNamespace(secretLocal.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Deleted
	}
	secretLocal.Namespace = nattedNs

	klog.V(3).Infof("PreDelete routine completed for secret %v/%v", secretLocal.Namespace, secretLocal.Name)
	return secretLocal, watch.Deleted
}

func (r *SecretsReflector) isAllowed(_ context.Context, obj interface{}) bool {
	sec, ok := obj.(*corev1.Secret)
	if !ok {
		klog.Error("cannot convert obj to secret")
		return false
	}
	// if this annotation is set, this secret will not be reflected to the remote cluster
	val, ok := sec.Annotations["liqo.io/not-reflect"]
	return !ok || val != "true"
}
