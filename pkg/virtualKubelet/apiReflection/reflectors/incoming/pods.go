package incoming

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
	ri "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors/reflectorsInterfaces"
	vkContext "github.com/liqotech/liqo/pkg/virtualKubelet/context"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
)

// PodsIncomingReflector is the incoming reflector in charge of detecting status change in foreign pods
// and pushing the updated object to the vk internals.
type PodsIncomingReflector struct {
	ri.APIReflector

	RemoteRemappedPodCIDR options.ReadOnlyOption
}

// SetSpecializedPreProcessingHandlers allows to set the pre-routine handlers for the PodsIncomingReflector.
func (r *PodsIncomingReflector) SetSpecializedPreProcessingHandlers() {
	r.SetPreProcessingHandlers(ri.PreProcessingHandlers{
		AddFunc:    r.PreAdd,
		UpdateFunc: r.PreUpdate,
		DeleteFunc: r.PreDelete,
		IsAllowed:  r.isAllowed,
	})
}

// HandleEvent is the final function call in charge of pushing the foreignPod to the vk internals.
func (r *PodsIncomingReflector) HandleEvent(e interface{}) {
	event := e.(watch.Event)
	pod, ok := event.Object.(*corev1.Pod)
	if !ok {
		klog.Error("INCOMING REFLECTION: cannot cast object to pod")
		return
	}

	if pod == nil {
		klog.V(4).Info("INCOMING REFLECTION: received nil pod to process")
		return
	}

	klog.V(3).Infof("INCOMING REFLECTION: received %v for pod %v", event.Type, pod.Name)

	r.PushToInforming(pod)
}

// PreAdd is the pre-routine called in case of pod creation in the foreign cluster. It returns the home object with its
// status updated.
func (r *PodsIncomingReflector) PreAdd(obj interface{}) (interface{}, watch.EventType) {
	foreignPod := obj.(*corev1.Pod)

	homePod := r.sharedPreRoutine(foreignPod)
	if homePod == nil {
		return nil, watch.Added
	}

	return homePod, watch.Added
}

// PreAdd is the pre-routine called in case of pod update in the foreign cluster. It returns the home object with its
// status updated.
func (r *PodsIncomingReflector) PreUpdate(newObj, _ interface{}) (interface{}, watch.EventType) {
	foreignPod := newObj.(*corev1.Pod)

	if foreignPod == nil {
		return nil, watch.Modified
	}

	homePod := r.sharedPreRoutine(foreignPod)
	if homePod == nil {
		return nil, watch.Modified
	}

	return homePod, watch.Modified
}

// sharedPreRoutine is a common function used by both PreAdd and PreUpdate. It is in charge of fetching the home pod from
// the internal caches, updating its status and returning to the calling function.
func (r *PodsIncomingReflector) sharedPreRoutine(foreignPod *corev1.Pod) *corev1.Pod {
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

	// forge.ForeignToHomeStatus blacklists the received pod if the deletionTimestamp is set
	homePod, err = forge.ForeignToHomeStatus(foreignPod, homePod.(runtime.Object).DeepCopyObject())
	if err != nil {
		klog.Error(err)
		return nil
	}

	return homePod.(*corev1.Pod)
}

// PreDelete removes the received object from the blacklist for freeing the occupied space.
func (r *PodsIncomingReflector) PreDelete(obj interface{}) (interface{}, watch.EventType) {
	foreignPod := obj.(*corev1.Pod)
	foreignKey := fmt.Sprintf("%s/%s", foreignPod.Namespace, foreignPod.Name)
	delete(reflectors.Blacklist[apimgmt.Pods], foreignKey)
	klog.V(3).Infof("pod %s removed from blacklist because deleted", foreignKey)

	return nil, watch.Deleted
}

// CleanupNamespace is in charge of cleaning a local namespace from all the reflected objects. All the home objects in
// the home namespace are fetched and deleted locally. Their deletion will implies the delete of the remote replicasets.
func (r *PodsIncomingReflector) CleanupNamespace(namespace string) {
	foreignNamespace, err := r.NattingTable().NatNamespace(namespace, false)
	if err != nil {
		klog.Error(err)
		return
	}

	objects, err := r.GetCacheManager().ListForeignNamespacedObject(apimgmt.Pods, foreignNamespace)
	if err != nil {
		klog.Errorf("error while listing foreign objects in namespace %v", foreignNamespace)
		return
	}

	retriable := func(err error) bool {
		switch kerrors.ReasonForError(err) {
		case metav1.StatusReasonNotFound:
			return false
		default:
			klog.Warningf("retrying while deleting pod because of ERR; %v", err)
			return true
		}
	}

	for _, obj := range objects {
		foreignPod := obj.(*corev1.Pod)
		if foreignPod.Labels == nil {
			continue
		}

		homePodName, ok := foreignPod.Labels[virtualKubelet.ReflectedpodKey]
		if !ok {
			continue
		}
		// allow deletion of the related homePod by removing its finalizer
		finalizerPatch := []byte(fmt.Sprintf(
			`[{"op":"remove","path":"/metadata/finalizers","value":["%s"]}]`,
			virtualKubelet.HomePodFinalizer))

		_, err = r.GetHomeClient().CoreV1().Pods(namespace).Patch(context.TODO(),
			homePodName,
			types.JSONPatchType,
			finalizerPatch,
			metav1.PatchOptions{})
		if err != nil {
			klog.Error(err)
			continue
		}

		if err := retry.OnError(retry.DefaultBackoff, retriable, func() error {
			return r.GetHomeClient().CoreV1().Pods(namespace).Delete(context.TODO(), homePodName, metav1.DeleteOptions{})
		}); err != nil {
			klog.Errorf("Error while deleting home pod %v/%v - ERR: %v", namespace, homePodName, err)
		}
	}
}

// isAllowed checks that the received object has to be processed by the reflector.
// if the event is a deletion, the reflector always handles it, because it has to remove the received object
// from the blacklist.
func (r *PodsIncomingReflector) isAllowed(ctx context.Context, obj interface{}) bool {
	if value, ok := vkContext.IncomingMethod(ctx); ok && value == vkContext.IncomingDeleted {
		return true
	}

	pod, ok := obj.(*corev1.Pod)
	if !ok {
		klog.Error("cannot convert obj to pod")
		return false
	}
	key := r.Keyer(pod.Namespace, pod.Name)
	_, ok = reflectors.Blacklist[apimgmt.Pods][key]
	if ok {
		klog.V(5).Infof("event for pod %v blacklisted", key)
	}
	return !ok
}
