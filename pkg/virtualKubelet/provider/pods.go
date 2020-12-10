package provider

import (
	"context"
	"fmt"
	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/liqotech/liqo/internal/utils/trace"
	"github.com/liqotech/liqo/internal/virtualKubelet/node/api"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmgt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	vkContext "github.com/liqotech/liqo/pkg/virtualKubelet/context"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/translation/serviceEnv"
	"github.com/pkg/errors"
	"io"
	corev1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	"math/rand"
	"time"
)

// CreatePod accepts a Pod definition and stores it in memory.
func (p *LiqoProvider) CreatePod(_ context.Context, homePod *corev1.Pod) error {
	if homePod == nil {
		return errors.New("received nil homePod to create")
	}

	klog.V(3).Infof("PROVIDER: pod %s/%s asked to be created in the provider", homePod.Namespace, homePod.Name)

	if homePod.OwnerReferences != nil && len(homePod.OwnerReferences) != 0 && homePod.OwnerReferences[0].Kind == "DaemonSet" {
		klog.Infof("Skip to create DaemonSet homePod %q", homePod.Name)
		return nil
	}

	foreignPod, err := forge.HomeToForeign(homePod, nil, forge.LiqoOutgoing)
	if err != nil {
		return err
	}

	foreignPod, err = serviceEnv.TranslateServiceEnvVariables(foreignPod.(*corev1.Pod), homePod.Namespace, homePod.Namespace, p.apiController.CacheManager())
	if err != nil {
		klog.Error(err)
	}

	foreignReplicaset := forge.ReplicasetFromPod(foreignPod.(*corev1.Pod))

	_, err = p.foreignClient.AppsV1().ReplicaSets(foreignReplicaset.Namespace).Create(context.TODO(), foreignReplicaset, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	klog.V(3).Infof("PROVIDER: replicaset %v/%v successfully created on remote cluster", foreignReplicaset.Namespace, foreignReplicaset.Name)

	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *LiqoProvider) UpdatePod(_ context.Context, pod *corev1.Pod) error {
	if pod == nil {
		return errors.New("received nil pod to update")
	}

	klog.V(3).Infof("PROVIDER: pod %s/%s asked to be updated in the provider", pod.Namespace, pod.Name)

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *LiqoProvider) DeletePod(ctx context.Context, pod *corev1.Pod) (err error) {
	if pod == nil {
		return errors.New("received nil pod to delete")
	}

	var foreignNamespace, replicasetName string

	klog.V(3).Infof("PROVIDER: pod %s/%s asked to be deleted in the provider", pod.Namespace, pod.Name)

	// if the caller of the functions is deleteDanglingPods, then the received pod is the foreign one,
	// otherwise the received pod is the local one
	if value, ok := vkContext.CallingFunction(ctx); ok && value == vkContext.DeleteDanglingPods {
		foreignNamespace = pod.Namespace
		if pod.Labels != nil {
			replicasetName = pod.Labels[virtualKubelet.ReflectedpodKey]
		}
		if replicasetName == "" {
			klog.V(3).Infof("foreign pod %s/%s not deleted because unlabeled", pod.Namespace, pod.Name)
			return nil
		}
	} else {
		replicasetName = pod.Name
		foreignNamespace, err = p.namespaceMapper.NatNamespace(pod.Namespace, false)
		if err != nil {
			return err
		}
	}

	err = p.foreignClient.AppsV1().ReplicaSets(foreignNamespace).Delete(context.TODO(), replicasetName, metav1.DeleteOptions{})
	if kerror.IsNotFound(err) {
		klog.V(5).Infof("replicaset %v/%v not deleted because not existing", foreignNamespace, replicasetName)
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "Unable to delete foreign replicaset")
	}

	klog.V(3).Infof("PROVIDER: replicaset %v/%v successfully deleted on remote cluster", foreignNamespace, pod.Name)

	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *LiqoProvider) GetPod(_ context.Context, namespace, name string) (pod *corev1.Pod, err error) {
	klog.V(3).Infof("PROVIDER: pod %s/%s requested to the provider", namespace, name)

	foreignNamespace, err := p.namespaceMapper.NatNamespace(namespace, false)
	if err != nil {
		return nil, err
	}

	foreignPod, err := p.apiController.CacheManager().GetForeignApiByIndex(apimgmgt.Pods, foreignNamespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "error while retrieving foreign pod")
	}

	return foreignPod.(*corev1.Pod), nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *LiqoProvider) GetPodStatus(_ context.Context, namespace, name string) (*corev1.PodStatus, error) {
	klog.V(3).Infof("PROVIDER: pod %s/%s status requested to the provider", namespace, name)

	foreignNamespace, err := p.namespaceMapper.NatNamespace(namespace, false)

	if err != nil {
		return nil, nil
	}

	foreignPod, err := p.apiController.CacheManager().GetForeignApiByIndex(apimgmgt.Pods, foreignNamespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "error while retrieving foreign pod")
	}

	return &foreignPod.(*corev1.Pod).Status, nil
}

// GetPods returns a list of all pods known to be "running".
func (p *LiqoProvider) GetPods(_ context.Context) ([]*corev1.Pod, error) {
	klog.V(3).Infof("PROVIDER: foreign pod listing requested to the provider")

	var homePods []*corev1.Pod

	for _, foreignNamespace := range p.namespaceMapper.MappedNamespaces() {
		pods, err := p.apiController.CacheManager().ListForeignNamespacedObject(apimgmgt.Pods, foreignNamespace)
		if err != nil {
			return nil, errors.New("Unable to get pods")
		}

		for _, pod := range pods {
			homePod, err := forge.ForeignToHome(pod.(*corev1.Pod), nil, forge.LiqoOutgoing)
			if err != nil {
				return nil, err
			}
			homePods = append(homePods, homePod.(*corev1.Pod))
		}
	}

	return homePods, nil
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *LiqoProvider) RunInContainer(_ context.Context, homeNamespace string, homePodName string, containerName string, cmd []string, attach api.AttachIO) error {

	foreignNamespace, err := p.namespaceMapper.NatNamespace(homeNamespace, false)
	if err != nil {
		return err
	}

	foreignObj, err := p.apiController.CacheManager().GetForeignApiByIndex(apimgmgt.Pods, foreignNamespace, homePodName)
	if err != nil {
		return errors.Wrap(err, "error while retrieving foreign pod")
	}
	foreignPod := foreignObj.(*corev1.Pod)

	req := p.foreignClient.CoreV1().RESTClient().
		Post().
		Namespace(foreignNamespace).
		Resource("pods").
		Name(foreignPod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command:   cmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(p.restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("could not make remote command: %v", err)
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  attach.Stdin(),
		Stdout: attach.Stdout(),
		Stderr: attach.Stderr(),
		Tty:    attach.TTY(),
	})
	if err != nil {
		return fmt.Errorf("streaming error: %v", err)
	}

	return nil
}

// GetContainerLogs retrieves the logs of a container by name from the provider.
func (p *LiqoProvider) GetContainerLogs(_ context.Context, homeNamespace string, homePodName string, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	foreignNamespace, err := p.namespaceMapper.NatNamespace(homeNamespace, false)
	if err != nil {
		return nil, err
	}

	foreignObj, err := p.apiController.CacheManager().GetForeignApiByIndex(apimgmgt.Pods, foreignNamespace, homePodName)
	if err != nil {
		return nil, errors.Wrap(err, "error while retrieving foreign pod")
	}
	foreignPod := foreignObj.(*corev1.Pod)

	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     opts.Follow,
		Previous:   opts.Previous,
		Timestamps: opts.Timestamps,
	}

	if opts.SinceSeconds > 0 {
		logOptions.SinceSeconds = &opts.SinceSeconds
	}
	if !opts.SinceTime.IsZero() {
		logOptions.SinceTime = &opts.SinceTime
	}
	if opts.LimitBytes > 0 {
		logOptions.LimitBytes = &opts.LimitBytes
	}
	if opts.Tail > 0 {
		logOptions.TailLines = &opts.Tail
	}

	logs := p.foreignClient.CoreV1().Pods(foreignNamespace).GetLogs(foreignPod.Name, logOptions)
	stream, err := logs.Stream(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("could not get stream from logs request: %v", err)
	}
	return stream, nil
}

// GetStatsSummary returns dummy stats for all pods known by this provider.
func (p *LiqoProvider) GetStatsSummary(ctx context.Context) (*stats.Summary, error) {
	var span trace.Span
	_, span = trace.StartSpan(ctx, "GetStatsSummary") //nolint: ineffassign
	defer span.End()

	// Grab the current timestamp so we can report it as the time the stats were generated.
	t := metav1.NewTime(time.Now())

	// Create the Summary object that will later be populated with node and pod stats.
	res := &stats.Summary{}

	// Populate the Summary object with basic node stats.
	res.Node = stats.NodeStats{
		NodeName:  p.nodeName.Value().ToString(),
		StartTime: metav1.NewTime(p.startTime),
	}

	// Populate the Summary object with dummy stats for each pod known by this provider.
	pods, err := p.foreignClient.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		if kerror.IsNotFound(err) {
			return nil, errdefs.NotFoundf("pods in \"%s\" is not known to the provider", "")
		}
		return nil, errors.Wrap(err, "Unable to get pods")
	}
	for _, pod := range pods.Items {
		var (
			// totalUsageNanoCores will be populated with the sum of the values of UsageNanoCores computes across all containers in the pod.
			totalUsageNanoCores uint64
			// totalUsageBytes will be populated with the sum of the values of UsageBytes computed across all containers in the pod.
			totalUsageBytes uint64
		)

		// Create a PodStats object to populate with pod stats.
		pss := stats.PodStats{
			PodRef: stats.PodReference{
				Name:      pod.Name,
				Namespace: pod.Namespace,
				UID:       string(pod.UID),
			},
			StartTime: pod.CreationTimestamp,
		}

		// Iterate over all containers in the current pod to compute dummy stats.
		for _, container := range pod.Spec.Containers {
			// Grab a dummy value to be used as the total CPU usage.
			// The value should fit a uint32 in order to avoid overflows later on when computing pod stats.
			dummyUsageNanoCores := uint64(rand.Uint32())
			totalUsageNanoCores += dummyUsageNanoCores
			// Create a dummy value to be used as the total RAM usage.
			// The value should fit a uint32 in order to avoid overflows later on when computing pod stats.
			dummyUsageBytes := uint64(rand.Uint32())
			totalUsageBytes += dummyUsageBytes
			// Append a ContainerStats object containing the dummy stats to the PodStats object.
			pss.Containers = append(pss.Containers, stats.ContainerStats{
				Name:      container.Name,
				StartTime: pod.CreationTimestamp,
				CPU: &stats.CPUStats{
					Time:           t,
					UsageNanoCores: &dummyUsageNanoCores,
				},
				Memory: &stats.MemoryStats{
					Time:       t,
					UsageBytes: &dummyUsageBytes,
				},
			})
		}

		// Populate the CPU and RAM stats for the pod and append the PodsStats object to the Summary object to be returned.
		pss.CPU = &stats.CPUStats{
			Time:           t,
			UsageNanoCores: &totalUsageNanoCores,
		}
		pss.Memory = &stats.MemoryStats{
			Time:       t,
			UsageBytes: &totalUsageBytes,
		}
		res.Pods = append(res.Pods, pss)
	}

	// Return the dummy stats.
	return res, nil
}

// NotifyPods is called to set a pod informing callback function. This should be called before any operations are ready
// within the provider.
func (p *LiqoProvider) NotifyPods(_ context.Context, notifier func(interface{})) {
	p.apiController.SetInformingFunc(apimgmgt.Pods, notifier)
	p.apiController.SetInformingFunc(apimgmgt.ReplicaSets, notifier)
}
