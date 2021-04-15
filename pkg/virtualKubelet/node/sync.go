package node

import (
	"context"
	"k8s.io/klog/v2"
	"sync"
	"time"

	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	podStatusReasonNotFound          = "NotFound"
	podStatusMessageNotFound         = "The pod status was not found and may have been deleted from the provider"
	containerStatusReasonNotFound    = "NotFound"
	containerStatusMessageNotFound   = "Container was not found and was likely deleted"
	containerStatusExitCodeNotFound  = -137
	statusTerminatedReason           = "Terminated"
	containerStatusTerminatedMessage = "Container was terminated. The exit code may not reflect the real exit code"
)

// syncProviderWrapper wraps a PodLifecycleHandler to give it async-like pod status notification behavior.
type syncProviderWrapper struct {
	PodLifecycleHandler
	notify func(interface{})
	l      corev1listers.PodLister

	// deletedPods makes sure we don't set the "NotFound" status
	// for pods which have been requested to be deleted.
	// This is needed for our loop which just grabs pod statuses every 5 seconds.
	deletedPods sync.Map
}

// This is used to clean up keys for deleted pods after they have been fully deleted in the API server.
type syncWrapper interface {
	_deletePodKey(context.Context, string)
}

func (p *syncProviderWrapper) NotifyPods(ctx context.Context, f func(interface{})) {
	p.notify = f
}

func (p *syncProviderWrapper) _deletePodKey(ctx context.Context, key string) {
	klog.V(4).Info("Cleaning up pod from deletion cache")
	p.deletedPods.Delete(key)
}

func (p *syncProviderWrapper) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	klog.V(4).Info("syncProviderWrappper.DeletePod")
	key, err := cache.MetaNamespaceKeyFunc(pod)
	if err != nil {
		return err
	}

	p.deletedPods.Store(key, pod)
	if err := p.PodLifecycleHandler.DeletePod(ctx, pod.DeepCopy()); err != nil {
		klog.V(4).Info("%v - Removed key %s from deleted pods cache", err, key)
		// We aren't going to actually delete the pod from the provider since there is an error so delete it from our cache,
		// otherwise we could end up leaking pods in our deletion cache.
		// Delete will be retried by the pod controller.
		p.deletedPods.Delete(key)
		return err
	}

	if shouldSkipPodStatusUpdate(pod) {
		klog.V(4).Info("skipping pod status update for terminated pod")
		return nil
	}

	updated := pod.DeepCopy()
	updated.Status.Phase = corev1.PodSucceeded
	now := metav1.NewTime(time.Now())
	for i, cs := range updated.Status.ContainerStatuses {
		updated.Status.ContainerStatuses[i].State.Terminated = &corev1.ContainerStateTerminated{
			Reason:     statusTerminatedReason,
			Message:    containerStatusTerminatedMessage,
			FinishedAt: now,
		}
		if cs.State.Running != nil {
			updated.Status.ContainerStatuses[i].State.Terminated.StartedAt = cs.State.Running.StartedAt
		}
	}
	updated.Status.Reason = statusTerminatedReason

	p.notify(updated)
	klog.V(4).Info("Notified pod terminal pod status")

	return nil
}

func (p *syncProviderWrapper) run(ctx context.Context) {
	interval := 5 * time.Second
	timer := time.NewTimer(interval)

	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	for {
		klog.V(4).Info("Pod status update loop start")
		timer.Reset(interval)
		select {
		case <-ctx.Done():
			klog.V(4).Info(ctx.Err(), " - sync wrapper loop exiting")
			return
		case <-timer.C:
		}
		p.syncPodStatuses(ctx)
	}
}

func (p *syncProviderWrapper) syncPodStatuses(ctx context.Context) {

	// Update all the pods with the provider status.
	pods, err := p.l.List(labels.Everything())
	if err != nil {
		err = errors.Wrap(err, "error getting pod list from kubernetes")
		klog.Error(err, "Error updating pod statuses")
		return
	}

	for _, pod := range pods {
		if shouldSkipPodStatusUpdate(pod) {
			klog.V(4).Info("Skipping pod status update")
			continue
		}

		if err := p.updatePodStatus(ctx, pod); err != nil {
			klog.Errorf("%v - Could not fetch pod %s/%s status", err, pod.Name, pod.Namespace)
		}
	}
}

func (p *syncProviderWrapper) updatePodStatus(ctx context.Context, podFromKubernetes *corev1.Pod) error {
	var statusErr error
	podStatus, err := p.PodLifecycleHandler.GetPodStatus(ctx, podFromKubernetes.Namespace, podFromKubernetes.Name)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
		statusErr = err
	}
	if podStatus != nil {
		pod := podFromKubernetes.DeepCopy()
		podStatus.DeepCopyInto(&pod.Status)
		p.notify(pod)
		return nil
	}

	key, err := cache.MetaNamespaceKeyFunc(podFromKubernetes)
	if err != nil {
		return err
	}

	if _, exists := p.deletedPods.Load(key); exists {
		klog.V(4).Info("pod is in known deleted state, ignoring")
		return nil
	}

	if podFromKubernetes.Status.Phase != corev1.PodRunning && time.Since(podFromKubernetes.ObjectMeta.CreationTimestamp.Time) <= time.Minute {
		return statusErr
	}

	// Only change the status when the pod was already up.
	// Only doing so when the pod was successfully running makes sure we don't run into race conditions during pod creation.
	// Set the pod to failed, this makes sure if the underlying container implementation is gone that a new pod will be created.
	podStatus = podFromKubernetes.Status.DeepCopy()
	podStatus.Phase = corev1.PodFailed
	podStatus.Reason = podStatusReasonNotFound
	podStatus.Message = podStatusMessageNotFound
	now := metav1.NewTime(time.Now())
	for i, c := range podStatus.ContainerStatuses {
		if c.State.Running == nil {
			continue
		}
		podStatus.ContainerStatuses[i].State.Terminated = &corev1.ContainerStateTerminated{
			ExitCode:    containerStatusExitCodeNotFound,
			Reason:      containerStatusReasonNotFound,
			Message:     containerStatusMessageNotFound,
			FinishedAt:  now,
			StartedAt:   c.State.Running.StartedAt,
			ContainerID: c.ContainerID,
		}
		podStatus.ContainerStatuses[i].State.Running = nil
	}

	klog.V(4).Info("Setting pod not found on pod status")
	pod := podFromKubernetes.DeepCopy()
	podStatus.DeepCopyInto(&pod.Status)
	p.notify(pod)
	return nil
}
