// Copyright 2019-2025 The Liqo Authors
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
	"io"
	"sync"

	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	corev1clients "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	metricsv1beta1 "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/utils/virtualkubelet"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/generic"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

var _ manager.Reflector = (*PodReflector)(nil)
var _ PodHandler = (*PodReflector)(nil)
var _ manager.FallbackReflector = (*FallbackPodReflector)(nil)

const (
	// PodReflectorName -> The name associated with the Pod reflector.
	PodReflectorName = "Pod"
)

// MetricsFactory represents a function to generate the interface to retrieve the pod metrics for a given namespace.
type MetricsFactory func(namespace string) metricsv1beta1.PodMetricsInterface

// PodHandler exposes an interface to interact with pods offloaded to the remote cluster.
type PodHandler interface {
	// List returns the list of reflected pods.
	List(context.Context) ([]*corev1.Pod, error)
	// Exec executes a command in a container of a reflected pod.
	Exec(ctx context.Context, namespace, pod, container string, cmd []string, attach api.AttachIO) error
	// Attach attaches to a process that is already running inside an existing container of a reflected pod.
	Attach(ctx context.Context, namespace, pod, container string, attach api.AttachIO) error
	// PortForward forwards a connection from local to the ports of a reflected pod.
	PortForward(ctx context.Context, namespace, pod string, port int32, stream io.ReadWriteCloser) error
	// Logs retrieves the logs of a container of a reflected pod.
	Logs(ctx context.Context, namespace, pod, container string, opts api.ContainerLogOpts) (io.ReadCloser, error)
	// Stats retrieves the stats of the reflected pods.
	Stats(ctx context.Context) (*statsv1alpha1.Summary, error)
}

// PodReflector manages the Pod reflection towards a remote cluster.
type PodReflector struct {
	manager.Reflector

	localPods corev1listers.PodLister

	remoteRESTConfig     *rest.Config
	remoteMetricsFactory MetricsFactory

	handlers sync.Map /* implicit signature: map[string]NamespacedPodHandler */

	config *PodReflectorConfig
}

// PodReflectorConfig represents the configuration of a PodReflector.
type PodReflectorConfig struct {
	APIServerSupport    forge.APIServerSupportType
	DisableIPReflection bool
	HomeAPIServerHost   string
	HomeAPIServerPort   string

	KubernetesServiceIPMapper func(context.Context) (string, error)
	NetConfiguration          *networkingv1beta1.Configuration
}

// FallbackPodReflector handles the "orphan" pods outside the managed namespaces.
type FallbackPodReflector struct {
	localPods       corev1listers.PodLister
	localPodsClient func(namespace string) corev1clients.PodInterface
	ready           func() bool
	recorder        record.EventRecorder
}

// String returns the name of the PodReflector.
func (pr *PodReflector) String() string {
	return PodReflectorName
}

// NewPodReflector returns a new PodReflector instance.
func NewPodReflector(
	remoteRESTConfig *rest.Config, /* required to establish the connection to implement `kubectl exec` */
	remoteMetricsFactory MetricsFactory, /* required to retrieve the pod metrics from the remote cluster */
	podReflectorconfig *PodReflectorConfig,
	reflectorConfig *offloadingv1beta1.ReflectorConfig) *PodReflector {
	reflector := &PodReflector{
		remoteRESTConfig:     remoteRESTConfig,
		remoteMetricsFactory: remoteMetricsFactory,
		config:               podReflectorconfig,
	}

	genericReflector := generic.NewReflector(PodReflectorName, reflector.NewNamespaced, reflector.NewFallback,
		reflectorConfig.NumWorkers, offloadingv1beta1.CustomLiqo, generic.ConcurrencyModeAll)
	reflector.Reflector = genericReflector
	return reflector
}

// NewNamespaced returns a new NamespacedPodReflector instance.
func (pr *PodReflector) NewNamespaced(opts *options.NamespacedOpts) manager.NamespacedReflector {
	var err error
	remote := opts.RemoteFactory.Core().V1().Pods()
	_, err = remote.Informer().AddEventHandler(opts.HandlerFactory(RemoteShadowNamespacedKeyer(opts.LocalNamespace, forge.LiqoNodeName)))
	utilruntime.Must(err)
	remoteShadow := opts.RemoteLiqoFactory.Offloading().V1beta1().ShadowPods()
	_, err = remoteShadow.Informer().AddEventHandler(opts.HandlerFactory(RemoteShadowNamespacedKeyer(opts.LocalNamespace, forge.LiqoNodeName)))
	utilruntime.Must(err)
	remoteSecrets := opts.RemoteFactory.Core().V1().Secrets()

	reflector := &NamespacedPodReflector{
		NamespacedReflector: generic.NewNamespacedReflector(opts, PodReflectorName),

		localPods:        pr.localPods.Pods(opts.LocalNamespace),
		remotePods:       remote.Lister().Pods(opts.RemoteNamespace),
		remoteShadowPods: remoteShadow.Lister().ShadowPods(opts.RemoteNamespace),
		remoteSecrets:    remoteSecrets.Lister().Secrets(opts.RemoteNamespace),

		localPodsClient:        opts.LocalClient.CoreV1().Pods(opts.LocalNamespace),
		remotePodsClient:       opts.RemoteClient.CoreV1().Pods(opts.RemoteNamespace),
		remoteShadowPodsClient: opts.RemoteLiqoClient.OffloadingV1beta1().ShadowPods(opts.RemoteNamespace),

		remoteRESTClient: opts.RemoteClient.CoreV1().RESTClient(),
		remoteRESTConfig: pr.remoteRESTConfig,
		remoteMetrics:    pr.remoteMetricsFactory(opts.RemoteNamespace),

		config:                    pr.config,
		kubernetesServiceIPGetter: pr.KubernetesServiceIPGetter(),
	}

	pr.handlers.Store(opts.LocalNamespace, NamespacedPodHandler(reflector))
	return reflector
}

// NewFallback returns a new FallbackReflector instance.
func (pr *PodReflector) NewFallback(opts *options.ReflectorOpts) manager.FallbackReflector {
	opts.LocalPodInformer.Informer().AddEventHandler(opts.HandlerFactory(generic.BasicKeyer()))
	return &FallbackPodReflector{
		localPods:       opts.LocalPodInformer.Lister(),
		localPodsClient: opts.LocalClient.CoreV1().Pods,
		ready:           opts.Ready,
		recorder: opts.EventBroadcaster.NewRecorder(scheme.Scheme,
			corev1.EventSource{Component: "liqo-pod-reflection"}),
	}
}

// Start starts the reflector.
func (pr *PodReflector) Start(ctx context.Context, opts *options.ReflectorOpts) {
	pr.localPods = opts.LocalPodInformer.Lister()
	pr.Reflector.Start(ctx, opts)
}

// StopNamespace stops the reflection for a given namespace.
func (pr *PodReflector) StopNamespace(local, remote string) {
	pr.handlers.Delete(local)
	pr.Reflector.StopNamespace(local, remote)
}

// List returns the list of reflected pods.
func (pr *PodReflector) List(_ context.Context) ([]*corev1.Pod, error) {
	return pr.localPods.List(labels.Everything())
}

// Exec executes a command in a container of a reflected pod.
func (pr *PodReflector) Exec(ctx context.Context, namespace, pod, container string, cmd []string, attach api.AttachIO) error {
	if handler, found := pr.handlers.Load(namespace); found {
		return handler.(NamespacedPodHandler).Exec(ctx, pod, container, cmd, attach)
	}
	return kerrors.NewNotFound(corev1.Resource(corev1.ResourcePods.String()), klog.KRef(namespace, pod).String())
}

// Attach attaches to a process that is already running inside an existing container of a reflected pod.
func (pr *PodReflector) Attach(ctx context.Context, namespace, pod, container string, attach api.AttachIO) error {
	if handler, found := pr.handlers.Load(namespace); found {
		return handler.(NamespacedPodHandler).Attach(ctx, pod, container, attach)
	}
	return kerrors.NewNotFound(corev1.Resource(corev1.ResourcePods.String()), klog.KRef(namespace, pod).String())
}

// PortForward forwards a connection from local to the ports of a reflected pod.
func (pr *PodReflector) PortForward(ctx context.Context, namespace, pod string, port int32, stream io.ReadWriteCloser) error {
	if handler, found := pr.handlers.Load(namespace); found {
		return handler.(NamespacedPodHandler).PortForward(ctx, pod, port, stream)
	}
	return kerrors.NewNotFound(corev1.Resource(corev1.ResourcePods.String()), klog.KRef(namespace, pod).String())
}

// Logs retrieves the logs of a container of a reflected pod.
func (pr *PodReflector) Logs(ctx context.Context, namespace, pod, container string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	if handler, found := pr.handlers.Load(namespace); found {
		return handler.(NamespacedPodHandler).Logs(ctx, pod, container, opts)
	}
	return nil, kerrors.NewNotFound(corev1.Resource(corev1.ResourcePods.String()), klog.KRef(namespace, pod).String())
}

// Stats retrieves the stats of the reflected pods.
func (pr *PodReflector) Stats(ctx context.Context) (*statsv1alpha1.Summary, error) {
	var pods []statsv1alpha1.PodStats
	var err error

	pr.handlers.Range(func(_, handler interface{}) bool {
		var stats []statsv1alpha1.PodStats
		stats, err = handler.(NamespacedPodHandler).Stats(ctx)
		pods = append(pods, stats...)
		return err == nil
	})

	if err != nil {
		return nil, err
	}

	return forge.LocalNodeStats(pods), nil
}

// KubernetesServiceIPGetter returns a function to retrieve the IP associated with the kubernetes.default service.
func (pr *PodReflector) KubernetesServiceIPGetter() func(ctx context.Context) (string, error) {
	var address string
	var lock sync.Mutex

	return func(ctx context.Context) (string, error) {
		lock.Lock()
		defer lock.Unlock()

		// If the address has already been saved in cache, then return it directly.
		if address != "" {
			return address, nil
		}

		var err error
		address, err = pr.config.KubernetesServiceIPMapper(ctx)
		if err != nil {
			return "", err
		}

		return address, nil
	}
}

// Handle operates as fallback to reconcile pod objects not managed by namespaced handlers.
func (fpr *FallbackPodReflector) Handle(ctx context.Context, key types.NamespacedName) error {
	tracer := trace.FromContext(ctx)

	// Retrieve the local object (only not found errors can occur).
	klog.V(4).Infof("Handling fallback management of local pod %q", klog.KRef(key.Namespace, key.Name))
	local, err := fpr.localPods.Pods(key.Namespace).Get(key.Name)
	utilruntime.Must(client.IgnoreNotFound(err))
	tracer.Step("Retrieved the local object")

	if kerrors.IsNotFound(err) {
		klog.V(4).Infof("Local pod %q already vanished", klog.KRef(key.Namespace, key.Name))
		return nil
	}

	// The local pod is being terminated, hence delete it.
	if !local.DeletionTimestamp.IsZero() {
		defer tracer.Step("Ensured the absence of the local terminating object")

		klog.V(4).Infof("Deleting terminating orphan local pod %q", klog.KObj(local))
		opts := metav1.NewDeleteOptions(0 /* trigger the effective deletion */)
		opts.Preconditions = metav1.NewUIDPreconditions(string(local.GetUID()))
		if err := fpr.localPodsClient(key.Namespace).Delete(ctx, key.Name, *opts); err != nil && !kerrors.IsNotFound(err) {
			klog.Errorf("Failed to delete orphan local terminated pod %q: %v", klog.KObj(local), err)
			fpr.recorder.Event(local, corev1.EventTypeWarning, forge.EventFailedDeletion, forge.EventFailedDeletionMsg(err))
			return err
		}
		klog.Infof("Local orphan pod %q successfully deleted", klog.KObj(local))
		return nil
	}

	// The local pod already completed correctly, hence no change shall be performed.
	if local.Status.Phase == corev1.PodSucceeded {
		return nil
	}

	// Otherwise, mark the pod as rejected (either Pending or Failed based on its previous status).
	phase := corev1.PodPending
	reason := forge.PodOffloadingBackOffReason

	// If the local pod was already running, mark it as Failed to cause its controller to recreate it.
	if local.Status.Phase != corev1.PodPending || len(local.Status.ContainerStatuses) > 0 {
		phase = corev1.PodFailed
		reason = forge.PodOffloadingAbortedReason
	}

	pod := forge.LocalRejectedPod(local, phase, reason)
	_, err = fpr.localPodsClient(key.Namespace).UpdateStatus(ctx, pod, metav1.UpdateOptions{FieldManager: forge.ReflectionFieldManager})
	if err != nil {
		klog.Errorf("Failed to mark local pod %q as %v (%v): %v", klog.KObj(local), phase, reason, err)
		fpr.recorder.Event(local, corev1.EventTypeWarning, forge.EventReflectionDisabled, forge.EventReflectionDisabledErrorMsg(key.Namespace, err))
		return err
	}

	klog.Infof("Pod %q successfully marked as %v (%v)", klog.KObj(local), phase, reason)
	fpr.recorder.Event(local, corev1.EventTypeWarning, forge.EventReflectionDisabled, forge.EventReflectionDisabledMsg(key.Namespace))
	tracer.Step("Updated the local pod status")
	return nil
}

// Keys returns a set of keys to be enqueued for fallback processing for the given namespace pair.
func (fpr *FallbackPodReflector) Keys(local, _ string) []types.NamespacedName {
	pods, err := fpr.localPods.Pods(local).List(labels.Everything())
	utilruntime.Must(err)

	keys := make([]types.NamespacedName, 0, len(pods))
	keyer := generic.BasicKeyer()
	for _, pod := range pods {
		keys = append(keys, keyer(pod)...)
	}
	return keys
}

// Ready returns whether the FallbackReflector is completely initialized.
func (fpr *FallbackPodReflector) Ready() bool {
	return fpr.ready()
}

// List returns the list of pods managed by the FallbackReflector.
func (fpr *FallbackPodReflector) List() ([]interface{}, error) {
	return virtualkubelet.List[virtualkubelet.Lister[*corev1.Pod], *corev1.Pod](
		fpr.localPods,
	)
}
