// Copyright 2019-2022 The Liqo Authors
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

package workload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vkv1alpha1clients "github.com/liqotech/liqo/pkg/client/clientset/versioned/typed/virtualkubelet/v1alpha1"
	vkv1alpha1listers "github.com/liqotech/liqo/pkg/client/listers/virtualkubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/utils/pod"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
)

var _ manager.NamespacedReflector = (*NamespacedPodReflector)(nil)
var _ NamespacedPodHandler = (*NamespacedPodReflector)(nil)

// NamespacedPodHandler exposes an interface to interact with pods offloaded to the remote cluster in a given namespace.
type NamespacedPodHandler interface {
	// Exec executes a command in a container of a reflected pod.
	Exec(ctx context.Context, pod, container string, cmd []string, attach api.AttachIO) error
	// Logs retrieves the logs of a container of a reflected pod.
	Logs(ctx context.Context, pod, container string, opts api.ContainerLogOpts) (io.ReadCloser, error)
	// Stats retrieves the stats of the reflected pods.
	Stats(ctx context.Context) ([]statsv1alpha1.PodStats, error)
}

// NamespacedPodReflector manages the Pod reflection for a given pair of local and remote namespaces.
type NamespacedPodReflector struct {
	generic.NamespacedReflector

	localPods        corev1listers.PodNamespaceLister
	remotePods       corev1listers.PodNamespaceLister
	remoteShadowPods vkv1alpha1listers.ShadowPodNamespaceLister

	localPodsClient        corev1clients.PodInterface
	remotePodsClient       corev1clients.PodInterface
	remoteShadowPodsClient vkv1alpha1clients.ShadowPodInterface

	remoteRESTClient rest.Interface
	remoteRESTConfig *rest.Config
	remoteMetrics    metricsv1beta1.PodMetricsInterface

	ipamclient       ipam.IpamClient
	remoteIpamClient ipam.IpamClient
	pods             sync.Map /* implicit signature: map[string]*PodInfo */
}

// PodInfo contains information about known pods.
type PodInfo struct {
	Restarts  int32
	RemoteUID types.UID

	OriginalIP   string
	TranslatedIP string
}

// Handle reconciles pod objects.
func (npr *NamespacedPodReflector) Handle(ctx context.Context, name string) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local and remote objects (only not found errors can occur).
	klog.V(4).Infof("Handling reflection of local pod %q (remote: %q)", npr.LocalRef(name), npr.RemoteRef(name))

	local, lerr := npr.localPods.Get(name)
	utilruntime.Must(client.IgnoreNotFound(lerr))
	localExists := !kerrors.IsNotFound(lerr)

	remote, rerr := npr.remotePods.Get(name)
	utilruntime.Must(client.IgnoreNotFound(rerr))
	remoteExists := !kerrors.IsNotFound(rerr)

	shadow, serr := npr.remoteShadowPods.Get(name)
	utilruntime.Must(client.IgnoreNotFound(serr))
	shadowExists := !kerrors.IsNotFound(serr)

	tracer.Step("Retrieved the local and remote objects")

	// Abort the reflection if the remote object is not managed by us, as we do not want to mutate others' objects.
	if (remoteExists && !forge.IsReflected(remote)) || (shadowExists && !forge.IsReflected(shadow)) {
		klog.Infof("Skipping reflection of local pod %q as remote already exists and is not managed by us", npr.LocalRef(name))
		return nil
	}
	tracer.Step("Performed the sanity checks")

	// The local pod does no longer exist. Ensure the shadowpod is absent from the remote cluster.
	if !localExists {
		defer tracer.Step("Ensured the absence of the remote object")
		if !kerrors.IsNotFound(serr) {
			klog.V(4).Infof("Deleting remote shadowpod %q, since local pod %q does no longer exist", npr.RemoteRef(name), npr.LocalRef(name))
			return npr.DeleteRemote(ctx, npr.remoteShadowPodsClient, "ShadowPod", name, shadow.GetUID())
		}

		klog.V(4).Infof("Local pod %q and remote shadowpod %q both vanished", npr.LocalRef(name), npr.RemoteRef(name))
		return nil
	}

	// The local pod is being terminated, hence mark also the remote one as to be deleted.
	if !local.DeletionTimestamp.IsZero() {
		// The remote pod has terminated, and we need to delete the local one.
		if !remoteExists && !shadowExists {
			defer tracer.Step("Ensured the absence of the local terminating object")

			klog.V(4).Infof("Deleting terminating local pod %q, since remote %q does no longer exist", npr.LocalRef(name), npr.RemoteRef(name))
			opts := metav1.NewDeleteOptions(0 /* trigger the effective deletion */)
			opts.Preconditions = metav1.NewUIDPreconditions(string(local.GetUID()))
			if err := npr.localPodsClient.Delete(ctx, name, *opts); err != nil && !kerrors.IsNotFound(err) {
				klog.Errorf("Failed to delete local terminated pod %q: %v", npr.LocalRef(name), err)
				return err
			}

			npr.ForgetPodInfo(name)
			klog.Infof("Local pod %q successfully deleted", npr.LocalRef(name))
			return nil
		}

		// The remote object is not yet terminating, trigger its deletion.
		if shadowExists && shadow.DeletionTimestamp.IsZero() {
			defer tracer.Step("Ensured the absence of the remote object")
			klog.V(4).Infof("Deleting remote shadowpod %q, since local pod %q is terminating", npr.RemoteRef(name), npr.LocalRef(name))
			return npr.DeleteRemote(ctx, npr.remoteShadowPodsClient, "ShadowPod", name, shadow.GetUID())
		}

		// If the remote is already terminating, we need to reflect the status to the local one.
		return npr.HandleStatus(ctx, local, remote)
	}

	// Do not offload the pod if it was previously rejected, as new copies should have already been re-created.
	if local.Status.Phase == corev1.PodFailed && local.Status.Reason == forge.PodOffloadingAbortedReason {
		// Ensure the corresponding remote shadowpod is not still present due to transients.
		if shadowExists && shadow.DeletionTimestamp.IsZero() {
			defer tracer.Step("Ensured the absence of the remote object")
			klog.Infof("Deleting remote shadowpod %q, since local pod %q has been previously rejected", npr.RemoteRef(name), npr.LocalRef(name))
			return npr.DeleteRemote(ctx, npr.remoteShadowPodsClient, "ShadowPod", name, shadow.GetUID())
		}

		klog.Infof("Skipping reflection of local pod %q as previously rejected", npr.LocalRef(name))
		return nil
	}

	// Ensure the local pod has the appropriate labels to mark it as offloaded.
	if err := npr.HandleLabels(ctx, local); err != nil {
		return err
	}

	// The local target is currently running, and it is necessary to enforce its presence in the remote cluster.
	target := forge.RemoteShadowPod(local, shadow, npr.RemoteNamespace())
	tracer.Step("Forged the remote pod")

	// If the remote pod does not exist, then create it.
	if !shadowExists {
		defer tracer.Step("Ensured the presence of the remote object")
		if _, err := npr.remoteShadowPodsClient.Create(ctx, target, metav1.CreateOptions{FieldManager: forge.ReflectionFieldManager}); err != nil {
			if kerrors.IsAlreadyExists(err) {
				klog.Infof("Remote shadowpod %q already exists (local pod: %q)", npr.RemoteRef(name), npr.LocalRef(name))
				return nil
			}
			klog.Errorf("Failed to create remote shadowpod %q (local pod: %q): %v", npr.RemoteRef(name), npr.LocalRef(name), err)
			return err
		}

		klog.Infof("Remote shadowpod %q successfully created (local: %q)", npr.RemoteRef(name), npr.LocalRef(name))
		tracer.Step("Created the remote shadowpod")
		// Whatever the outcome, here we return without updating the status, as it will be triggered by future events.
		return nil
	}

	// Check whether it is necessary to update the shadowpod.
	needsUpdate := !labels.Equals(shadow.ObjectMeta.GetLabels(), target.ObjectMeta.GetLabels()) ||
		!labels.Equals(shadow.ObjectMeta.GetAnnotations(), target.ObjectMeta.GetAnnotations()) ||
		!pod.IsPodSpecEqual(&shadow.Spec.Pod, &target.Spec.Pod)
	tracer.Step("Checked whether a shadowpod update was needed")

	// If so, perform the actual update operation.
	if needsUpdate {
		_, rerr = npr.remoteShadowPodsClient.Update(ctx, target, metav1.UpdateOptions{FieldManager: forge.ReflectionFieldManager})
		if rerr != nil {
			klog.Errorf("Failed to update remote shadowpod %q (local pod: %q): %v", npr.RemoteRef(name), npr.LocalRef(name), rerr)
			return rerr
		}

		klog.Infof("Remote shadowpod %q successfully updated (local pod: %q)", npr.RemoteRef(name), npr.LocalRef(name))
		tracer.Step("Updated the remote shadowpod")
	} else {
		klog.V(4).Infof("Skipping remote shadowpod %q update, as already synced", npr.RemoteRef(name))
	}

	// Reflect the status from the remote pod to the local one.
	return npr.HandleStatus(ctx, local, remote)
}

// HandleLabels mutates the local object labels, to mark the pod as offloaded and allow filtering at the informer level.
func (npr *NamespacedPodReflector) HandleLabels(ctx context.Context, local *corev1.Pod) error {
	// Forge the mutation to be applied to the local pod.
	mutation, needsUpdate := forge.LocalPodOffloadedLabel(local)
	if !needsUpdate {
		klog.V(4).Infof("Skipping local pod %q labels update, as already synced", npr.RemoteRef(local.GetName()))
		return nil
	}

	defer trace.FromContext(ctx).Step("Updated the local pod labels")
	if _, err := npr.localPodsClient.Apply(ctx, mutation, forge.ApplyOptions()); err != nil {
		klog.Errorf("Failed to enforce local pod %q labels: %v", npr.LocalRef(local.GetName()), err)
		return err
	}

	klog.Infof("Local pod %q labels successfully enforced", npr.LocalRef(local.GetName()))
	return nil
}

// HandleStatus reflects the status from the remote Pod to the local one.
func (npr *NamespacedPodReflector) HandleStatus(ctx context.Context, local, remote *corev1.Pod) error {
	// Do not handle the status in case the remote pod has not yet been created, or already terminated.
	if remote == nil {
		return nil
	}

	tracer := trace.FromContext(ctx)
	info := npr.RetrievePodInfo(local.GetName())

	// Wrap the address translation logic, so that we do not have to handle errors in the forge logic.
	var terr error
	translator := func(original string) (translation string) {
		translation, terr = npr.MapPodIP(ctx, info, original)
		return translation
	}

	// Increase the restart count in case the remote pod changed.
	if info.RemoteUID != remote.GetUID() {
		if info.RemoteUID != "" {
			info.Restarts++
		} else {
			info.Restarts = npr.InferAdditionalRestarts(&local.Status, &remote.Status)
		}
		info.RemoteUID = remote.GetUID()
	}

	// Forge the local pod object to update its status.
	po := forge.LocalPod(local, remote, translator, info.Restarts)
	tracer.Step("Forged the local pod status")

	// Check whether an error occurred during address translation.
	if terr != nil {
		klog.Errorf("Reflection of local pod %q to %q failed: %v", npr.LocalRef(local.GetName()), npr.RemoteRef(local.GetName()), terr)
		return terr
	}

	// Do not attempt to perform an update if not necessary.
	if reflect.DeepEqual(local.Status, po.Status) {
		klog.V(4).Infof("Skipping local pod %q status update, as already synced", npr.LocalRef(local.GetName()))
		tracer.Step("Checked whether the local pod status update was necessary")
		return nil
	}
	tracer.Step("Checked whether the local pod status update was necessary")

	// Perform the status update.
	_, err := npr.localPodsClient.UpdateStatus(ctx, po, metav1.UpdateOptions{FieldManager: forge.ReflectionFieldManager})
	if err != nil {
		klog.Errorf("Failed to update local pod status %q (remote: %q): %v", npr.LocalRef(local.GetName()), npr.RemoteRef(local.GetName()), err)
		return err
	}

	klog.Infof("Local pod %q status successfully updated (remote: %q)", npr.LocalRef(local.GetName()), npr.RemoteRef(local.GetName()))
	tracer.Step("Updated the local pod status")
	return nil
}

// Exec executes a command in a container of a reflected pod.
func (npr *NamespacedPodReflector) Exec(ctx context.Context, po, container string, cmd []string, attach api.AttachIO) error {
	klog.V(4).Infof("Requested to exec command in container %q of local pod %q (remote %q)", container, npr.LocalRef(po), npr.RemoteRef(po))

	request := npr.remoteRESTClient.Post().
		Resource(corev1.ResourcePods.String()).
		Namespace(npr.RemoteNamespace()).
		Name(po).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   cmd,
			Stdin:     attach.Stdin() != nil,
			Stdout:    attach.Stdout() != nil,
			Stderr:    attach.Stderr() != nil,
			TTY:       attach.TTY(),
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(npr.remoteRESTConfig, http.MethodPost, request.URL())
	if err != nil {
		klog.Errorf("Failed to exec command in container %q of local pod %q (remote %q): %v", container, npr.LocalRef(po), npr.RemoteRef(po), err)
		return fmt.Errorf("failed to execute command: %w", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  attach.Stdin(),
		Stdout: attach.Stdout(),
		Stderr: attach.Stderr(),
		Tty:    attach.TTY(),
	})
	if err != nil {
		klog.Errorf("Failed to exec command in container %q of local pod %q (remote %q): %v", container, npr.LocalRef(po), npr.RemoteRef(po), err)
		return fmt.Errorf("failed to execute command: %w", err)
	}

	klog.Infof("Command in container %q in local pod %q (remote %q) successfully executed", container, npr.LocalRef(po), npr.RemoteRef(po))
	return nil
}

// Logs retrieves the logs of a container of a reflected pod.
func (npr *NamespacedPodReflector) Logs(ctx context.Context, po, container string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	klog.V(4).Infof("Requested logs of container %q of local pod %q (remote %q)", container, npr.LocalRef(po), npr.RemoteRef(po))

	Int64AsPointerOrNil := func(value int64) *int64 {
		if value != 0 {
			return &value
		}
		return nil
	}

	TimeAsPointerOrNil := func(value time.Time) *metav1.Time {
		if !value.IsZero() {
			output := metav1.NewTime(value)
			return &output
		}
		return nil
	}

	logOpts := corev1.PodLogOptions{
		Container:    container,
		Follow:       opts.Follow,
		LimitBytes:   Int64AsPointerOrNil(int64(opts.LimitBytes)),
		Previous:     opts.Previous,
		SinceSeconds: Int64AsPointerOrNil(int64(opts.SinceSeconds)),
		TailLines:    Int64AsPointerOrNil(int64(opts.Tail)),
		Timestamps:   opts.Timestamps,
		SinceTime:    TimeAsPointerOrNil(opts.SinceTime),
	}

	stream, err := npr.remotePodsClient.GetLogs(po, &logOpts).Stream(ctx)
	if err != nil {
		klog.Errorf("Failed to retrieve logs of container %q of local pod %q (remote %q): %v", container, npr.LocalRef(po), npr.RemoteRef(po), err)
		return nil, fmt.Errorf("could not get stream from logs request: %w", err)
	}
	klog.Infof("Logs of container %q of local pod %q (remote %q) successfully retrieved", container, npr.LocalRef(po), npr.RemoteRef(po))
	return stream, nil
}

// Stats retrieves the stats of the reflected pods.
func (npr *NamespacedPodReflector) Stats(ctx context.Context) ([]statsv1alpha1.PodStats, error) {
	klog.V(4).Infof("Requested to retrieve stats for local namespace %q (remote %q)", npr.LocalNamespace(), npr.RemoteNamespace())
	var stats []statsv1alpha1.PodStats

	// Retrieve all metrics from the remote namespace.
	metrics, err := npr.remoteMetrics.List(ctx, metav1.ListOptions{LabelSelector: forge.ReflectedLabelSelector().String()})
	if err != nil {
		return nil, errors.Wrapf(err, "error while listing remote pod metrics in namespace %q", npr.RemoteNamespace())
	}

	for idx := range metrics.Items {
		name := metrics.Items[idx].GetName()

		// Retrieve the local pod corresponding to the remote metrics.
		local, err := npr.localPods.Get(name)
		if err != nil {
			return nil, errors.Wrapf(err, "error while retrieving local pod %q", npr.LocalRef(name))
		}

		// Construct the stats for the local object and add them to the list.
		stats = append(stats, forge.LocalPodStats(local, &metrics.Items[idx]))
	}

	klog.Infof("Stats for local namespace %q (remote %q) correctly retrieved", npr.LocalNamespace(), npr.RemoteNamespace())
	return stats, nil
}

// RetrievePodInfo retrieves the pod information regarding a given pod.
func (npr *NamespacedPodReflector) RetrievePodInfo(po string) *PodInfo {
	info, _ := npr.pods.LoadOrStore(po, &PodInfo{})
	return info.(*PodInfo)
}

// ForgetPodInfo forgets about a pod and deletes the cached information.
func (npr *NamespacedPodReflector) ForgetPodInfo(po string) {
	npr.pods.Delete(po)
}

// MapPodIP maps the remote Pod address to the corresponding local one.
func (npr *NamespacedPodReflector) MapPodIP(ctx context.Context, info *PodInfo, original string) (string, error) {
	// Check the pod information whether a translation already exists for the given IP.
	// Let check if the original IP is the expected one, to avoid issues in case the remote IP changed.
	if info.OriginalIP == original {
		return info.TranslatedIP, nil
	}

	// Cache miss -> we need to interact with the IPAM to request the translation.
	// If the IP is for an endpoint on the remote cluster, then use the remote IPAM client to map it.
	// Otherwise (if it points to a real pod on the remote cluster), use GetHomePodIP.
	isOnRemoteIpam, err := npr.isOnRemoteIpam(ctx, original)
	if err != nil {
		return "", err
	}
	if isOnRemoteIpam {
		klog.V(6).Infof("Translating pod IP %v with remote IPAM", original)
		response, err := npr.remoteIpamClient.MapEndpointIP(ctx, &ipam.MapRequest{ClusterID: forge.LocalClusterID, Ip: original})
		if err != nil {
			return "", fmt.Errorf("failed to translate pod IP %v with remote IPAM: %w", original, err)
		}
		info.TranslatedIP = response.GetIp()
	} else {
		klog.V(6).Infof("Translating pod IP %v with local IPAM", original)
		response, err := npr.ipamclient.GetHomePodIP(ctx, &ipam.GetHomePodIPRequest{ClusterID: forge.RemoteClusterID, Ip: original})
		if err != nil {
			return "", fmt.Errorf("failed to translate pod IP %v with local IPAM: %w", original, err)
		}
		info.TranslatedIP = response.GetHomeIP()
	}

	info.OriginalIP = original

	klog.V(6).Infof("Translated remote pod IP %v to local %v", original, info.TranslatedIP)

	return info.TranslatedIP, nil
}

// isOnRemoteIpam checks if the given IP must be translated using the remote IPAM service.
func (npr *NamespacedPodReflector) isOnRemoteIpam(ctx context.Context, ip string) (bool, error) {
	if npr.remoteIpamClient == nil {
		return false, nil
	}
	belongs, err := npr.remoteIpamClient.BelongsToPodCIDR(ctx, &ipam.BelongsRequest{Ip: ip})
	if err != nil {
		return false, fmt.Errorf("BelongsPodCIDR failed: %w", err)
	}
	return !belongs.GetBelongs(), nil
}

// InferAdditionalRestarts estimates the number of remote pod restarts comparing the previously configured statues.
func (npr *NamespacedPodReflector) InferAdditionalRestarts(local, remote *corev1.PodStatus) int32 {
	// This local pod status has not been initialized yet.
	if len(local.ContainerStatuses) == 0 {
		return 0
	}

	// Lookup two matching containers and estimate the restarts.
	status := &local.ContainerStatuses[0]
	for idx := range remote.ContainerStatuses {
		if status.Name == remote.ContainerStatuses[idx].Name {
			if difference := status.RestartCount - remote.ContainerStatuses[idx].RestartCount; difference > 0 {
				return difference
			}
			// This might occur in case the local pod status is not up-to-date, and a restart occurred remotely.
			return 0
		}
	}

	// This should never occur, since the containers should match
	return 0
}
