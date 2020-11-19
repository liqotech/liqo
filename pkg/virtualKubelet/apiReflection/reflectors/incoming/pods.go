package incoming

import (
	"context"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
)

// PodsIncomingReflector is the incoming reflector in charge of detecting status change in foreign pods
// and pushing the updated object to the vk internals
type PodsIncomingReflector struct {
	ri.APIReflector

	RemoteRemappedPodCIDR options.ReadOnlyOption
}

// SetSpecializedPreProcessingHandlers allows to set the pre-routine handlers for the PodsIncomingReflector
func (r *PodsIncomingReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete})
}

// HandleEvent is the final function call in charge of pushing the foreignPod to the vk internals
func (r *PodsIncomingReflector) HandleEvent(e interface{}) {
	event := e.(watch.Event)
	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		klog.Error("INCOMING REFLECTION: cannot cast object to pod")
		return
	}

	klog.V(3).Infof("INCOMING REFLECTION: received %v for pod %v", event.Type, pod.Name)

	r.PushToInforming(pod)
}

// PreAdd is the pre-routine called in case of pod creation in the foreign cluster. It returns the home object with its
// status updated
func (r *PodsIncomingReflector) PreAdd(obj interface{}) interface{} {
	foreignPod := obj.(*corev1.Pod)

	homePod := r.preAddUpdate(foreignPod)
	if homePod == nil {
		return nil
	}

	return homePod
}

// PreAdd is the pre-routine called in case of pod update in the foreign cluster. It returns the home object with its
// status updated
func (r *PodsIncomingReflector) PreUpdate(newObj, _ interface{}) interface{} {
	foreignPod := newObj.(*corev1.Pod)

	if foreignPod == nil {
		return nil
	}

	homePod := r.preAddUpdate(foreignPod)
	if homePod == nil {
		return nil
	}

	return homePod
}

// preAddUpdate is a common function used by both PreAdd and PreUpdate. It is in charge of fetching the home pod from
// the internal caches, updating its status and returning to the calling function
func (r *PodsIncomingReflector) preAddUpdate(foreignPod *corev1.Pod) *corev1.Pod {
	if foreignPod.Labels == nil {
		return nil
	}

	homePodName, ok := foreignPod.Labels[virtualKubelet.ReflectedpodKey]
	if !ok {
		return nil
	}

	homeNamespace, err := r.NattingTable().DeNatNamespace(foreignPod.Namespace)
	if err != nil {
		klog.Error(err)
		return nil
	}

	homePod, err := r.GetCacheManager().GetHomeNamespacedObject(apimgmt.Pods, homeNamespace, homePodName)
	if err != nil {
		err = errors.Wrap(err, "local pod not found, incoming update blocked")
		klog.V(4).Info(err)
		return nil
	}

	homePod, err = forge.ForeignToHomeStatus(foreignPod, homePod.(runtime.Object).DeepCopyObject())
	if err != nil {
		klog.Error(err)
		return nil
	}

	return homePod.(*corev1.Pod)
}

// TODO: add here custom unavailable status
func (r *PodsIncomingReflector) PreDelete(_ interface{}) interface{} {
	return nil
}

// CleanupNamespace is in charge of cleaning a local namespace from all the reflected objects. All the home objects in
// the home namespace are fetched and deleted locally. Their deletion will implies the delete of the remote replicasets
func (r *PodsIncomingReflector) CleanupNamespace(namespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(namespace, false)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ResyncListForeignNamespacedObject(apimgmt.Pods, foreignNamespace)
	if err != nil {
		klog.Errorf("error while listing remote objects in namespace %v", namespace)
		return
	}

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting pod because of- ERR; %v", err)
			return true
		}
	}
	for _, obj := range objects {
		pod := obj.(*corev1.Pod)
		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetHomeClient().CoreV1().Pods(namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting remote pod %v/%v - ERR: %v", namespace, pod.Name, err)
		}
	}
}
