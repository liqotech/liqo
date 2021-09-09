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

package incoming

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// ReplicaSetsIncomingReflector is in charge of reflecting remote replicasets status change in the home cluster.
type ReplicaSetsIncomingReflector struct {
	ri.APIReflector
}

func (r *ReplicaSetsIncomingReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		AddFunc:    r.preAdd,
		UpdateFunc: r.preUpdate,
		DeleteFunc: r.preDelete,
	})
}

// HandleEvent takes the replicaset event and performs an operation in the home cluster.
// The only event to be handled by this reflector is the deletion of a replicaset
// Once a delete event is received, the object embedded in the received struct is a pod
// to be deleted in the home cluster.
func (r *ReplicaSetsIncomingReflector) HandleEvent(obj interface{}) {
	event, ok := obj.(watch.Event)
	if !ok {
		klog.Error("cannot cast object to event")
		return
	}

	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		klog.Error("INCOMING REFLECTION: wrong type, cannot cast object to pod")
		return
	}

	klog.V(3).Infof("INCOMING REFLECTION: received %v for pod %v/%v", event.Type, pod.Namespace, pod.Name)

	switch event.Type {
	case watch.Added, watch.Modified:
		klog.V(4).Infof("INCOMING REFLECTION: event %v for object %v/%v ignored", event.Type, pod.Namespace, pod.Name)
	case watch.Deleted:
		// if the event is a delete we enqueue a shadow pod having all the containers status terminated,
		// for allowing the replicasetController to collect it
		r.PushToInforming(pod)
		klog.V(3).Infof("INCOMING REFLECTION: delete for replicaset related to home pod %v/%v processed", pod.Namespace, pod.Name)
	}
}

// preAdd returns always nil because the add events have to be ignored.
func (r *ReplicaSetsIncomingReflector) preAdd(_ interface{}) (interface{}, watch.EventType) {
	return nil, watch.Added
}

// preUpdate returns always nil because the add events have to be ignored.
func (r *ReplicaSetsIncomingReflector) preUpdate(_, _ interface{}) (interface{}, watch.EventType) {
	return nil, watch.Modified
}

// preDelete receives a replicaset, then gets the home pod named according to a replicaset label,
// finally the pod is returned.
func (r *ReplicaSetsIncomingReflector) preDelete(obj interface{}) (interface{}, watch.EventType) {
	foreignReplicaSet := obj.(*appsv1.ReplicaSet).DeepCopy()

	homeNamespace, err := r.NattingTable().DeNatNamespace(foreignReplicaSet.Namespace)
	if err != nil {
		klog.Error(err)
		return nil, watch.Deleted
	}

	podName := foreignReplicaSet.Labels[virtualKubelet.ReflectedpodKey]
	if podName == "" {
		klog.V(4).Infof("INCOMING REFLECTION: label missing for replicaset %v/%v", foreignReplicaSet.Namespace, foreignReplicaSet.Name)
		return nil, watch.Deleted
	}

	homeObj, err := r.GetCacheManager().GetHomeNamespacedObject(apimgmt.Pods, homeNamespace, podName)
	if err != nil {
		klog.Error(err)
		return nil, watch.Deleted
	}

	homePod := homeObj.(*corev1.Pod).DeepCopy()

	// allow deletion of the related homePod by removing its finalizer
	finalizerPatch := []byte(fmt.Sprintf(
		`[{"op":"remove","path":"/metadata/finalizers","value":["%s"]}]`,
		virtualKubelet.HomePodFinalizer))

	_, err = r.GetHomeClient().CoreV1().Pods(homePod.Namespace).Patch(context.TODO(),
		homePod.Name,
		types.JSONPatchType,
		finalizerPatch,
		metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return nil, watch.Deleted
	}

	// if the DeletionTimestamp is already set, the replicaset deletion has been triggered by a homePod delete event,
	// hence we have not to delete it
	if homePod.DeletionTimestamp != nil {
		return nil, watch.Deleted
	}

	// if a foreign replicaset has been deleted, first we trigger a delete event for the home pod
	if err := r.GetHomeClient().CoreV1().Pods(homeNamespace).Delete(context.TODO(), podName, metav1.DeleteOptions{}); err != nil {
		klog.Errorf("INCOMING REFLECTION: error while deleting home pod %s/%s", homeNamespace, podName)
		return nil, watch.Deleted
	}

	// then we set all the containers in terminated status
	homePod = forge.ForeignReplicasetDeleted(homePod)

	return homePod, watch.Deleted
}

// CleanupNamespace does nothing because the delete of the remote replicasets is already triggered by
// pods incoming reflector with its CleanupNamespace implementation.
func (r *ReplicaSetsIncomingReflector) CleanupNamespace(namespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(namespace)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ListForeignNamespacedObject(apimgmt.ReplicaSets, foreignNamespace)
	if err != nil {
		klog.Errorf("error while listing remote objects in namespace %v", namespace)
		return
	}

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting rs because of ERR; %v", err)
			return true
		}
	}
	for _, obj := range objects {
		rs := obj.(*appsv1.ReplicaSet)
		if rs.Labels == nil {
			continue
		}
		if _, ok := rs.Labels[forge.LiqoOutgoingKey]; !ok {
			continue
		}
		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetForeignClient().AppsV1().ReplicaSets(foreignNamespace).Delete(context.TODO(), rs.Name, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting remote replicaset %v/%v - ERR: %v", namespace, rs.Name, err)
		}
	}
}
