package incoming

import (
	"context"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
)

// ReplicaSetsIncomingReflector is in charge of reflecting remote replicasets status change in the home cluster
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
// to be deleted in the home cluster
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
		if err := r.GetHomeClient().CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("INCOMING REFLECTION: error while deleting home pod %v/%v - ERR: %v", pod.Namespace, pod.Name, err)
		} else {
			klog.V(3).Infof("INCOMING REFLECTION: home pod %v/%v correctly deleted", pod.Namespace, pod.Name)
		}
	}
}

// preAdd returns always nil beacuse the add events have to be ignored
func (r *ReplicaSetsIncomingReflector) preAdd(_ interface{}) interface{} {
	return nil
}

// preUpdate returns always nil beacuse the add events have to be ignored
func (r *ReplicaSetsIncomingReflector) preUpdate(_, _ interface{}) interface{} {
	return nil
}

// preDelete receives a replicaset, then gets the home pod named according to a replicaset label,
// finally the pod is returned
func (r *ReplicaSetsIncomingReflector) preDelete(obj interface{}) interface{} {
	foreignReplicaSet := obj.(*appsv1.ReplicaSet).DeepCopy()

	homeNamespace, err := r.NattingTable().DeNatNamespace(foreignReplicaSet.Namespace)
	if err != nil {
		klog.Error(err)
		return nil
	}

	podName := foreignReplicaSet.Labels[virtualKubelet.ReflectedpodKey]
	if podName == "" {
		klog.V(4).Infof("INCOMING REFLECTION: label missing for replicaset %v/%v", foreignReplicaSet.Namespace, foreignReplicaSet.Name)
		return nil
	}

	po, err := r.GetCacheManager().GetHomeNamespacedObject(apimgmt.Pods, homeNamespace, podName)
	if err != nil {
		klog.Error(err)
		return nil
	}

	return po
}

// CleanupNamespace does nothing because the delete of the remote replicasets is already triggered by
// pods incoming reflector with its CleanupNamespace implementation.
func (r *ReplicaSetsIncomingReflector) CleanupNamespace(_ string) {}
