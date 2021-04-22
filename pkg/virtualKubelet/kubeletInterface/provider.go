package kubeletInterface

import (
	"context"
	"github.com/liqotech/liqo/pkg/virtualKubelet/node/module"
	"github.com/liqotech/liqo/pkg/virtualKubelet/node/module/api"
	"io"

	corev1 "k8s.io/api/core/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

// PodLifecycleHandler defines the interface used by the PodController to react
// to new and changed pods scheduled to the node that is being managed.
type PodLifecycleHandler interface {
	// CreatePod takes a Kubernetes Pod and deploys it within the provider.
	CreatePod(ctx context.Context, pod *corev1.Pod) error

	// UpdatePod takes a Kubernetes Pod and updates it within the provider.
	UpdatePod(ctx context.Context, pod *corev1.Pod) error

	// DeletePod takes a Kubernetes Pod and deletes it from the provider. Once a pod is deleted, the provider is
	// expected to call the NotifyPods callback with a terminal pod status where all the containers are in a terminal
	// state, as well as the pod. DeletePod may be called multiple times for the same pod.
	DeletePod(ctx context.Context, pod *corev1.Pod) error

	// GetPod retrieves a pod by name from the provider (can be cached).
	// The Pod returned is expected to be immutable, and may be accessed
	// concurrently outside of the calling goroutine. Therefore it is recommended
	// to return a version after DeepCopy.
	GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error)

	// GetPodStatus retrieves the status of a pod by name from the provider.
	// The PodStatus returned is expected to be immutable, and may be accessed
	// concurrently outside of the calling goroutine. Therefore it is recommended
	// to return a version after DeepCopy.
	GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error)

	// GetPods retrieves a list of all pods running on the provider (can be cached).
	// The Pods returned are expected to be immutable, and may be accessed
	// concurrently outside of the calling goroutine. Therefore it is recommended
	// to return a version after DeepCopy.
	GetPods(context.Context) ([]*corev1.Pod, error)
}

// PodNotifier is used as an extension to PodLifecycleHandler to support async updates of pod statuses.
type PodNotifier interface {
	// NotifyPods instructs the notifier to call the passed in function when
	// the pod status changes. It should be called when a pod's status changes.
	//
	// The provided pointer to a Pod is guaranteed to be used in a read-only
	// fashion. The provided pod's PodStatus should be up to date when
	// this function is called.
	//
	// NotifyPods will not block callers.
	NotifyPods(context.Context, func(interface{}))
}

// Provider contains the methods required to implement a virtual-kubelet provider
type Provider interface {
	PodLifecycleHandler

	// GetContainerLogs retrieves the logs of a container by name from the provider.
	GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error)

	// RunInContainer executes a command in a container in the pod, copying data
	// between in/out/err and the container's stdin/stdout/stderr.
	RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error

	// ConfigureNode enables a provider to configure the node object that
	// will be used for Kubernetes.
	ConfigureNode(context.Context, *corev1.Node)

	StartNodeUpdater(nodeRunner *module.NodeController) (chan struct{}, chan struct{}, error)
}

// PodMetricsProvider is an optional interface that providers can implement to expose pod stats
type PodMetricsProvider interface {
	GetStatsSummary(context.Context) (*stats.Summary, error)
}
