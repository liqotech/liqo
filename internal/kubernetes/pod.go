package kubernetes

import (
	"context"
	"fmt"
	"github.com/netgroup-polito/dronev2/internal/errdefs"
	"github.com/netgroup-polito/dronev2/internal/log"
	"github.com/netgroup-polito/dronev2/internal/node/api"
	"github.com/netgroup-polito/dronev2/internal/trace"
	"github.com/pkg/errors"
	"io"
	v1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	"math/rand"
	"strings"
	"time"
)

// CreatePod accepts a Pod definition and stores it in memory.
func (p *KubernetesProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {

	ctx, span := trace.StartSpan(ctx, "CreatePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Infof("receive CreatePod %q", pod.Name)

	if pod == nil {
		return errors.New("pod cannot be nil")
	}

	if pod.OwnerReferences != nil && len(pod.OwnerReferences) != 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
		msg := fmt.Sprintf("Skip to create DaemonSet pod %q", pod.Name)
		log.G(ctx).WithField("Method", "CreatePod").Info(msg)
		return nil
	}

	nattedNS := p.NatNamespace(pod.Namespace, true)

	if err := p.CreateNamespaceIfNotExisting(nattedNS); err != nil {
		return err
	}

	podTranslated := H2FTranslate(pod, nattedNS)

	podServer, err := p.foreignClient.CoreV1().Pods(podTranslated.Namespace).Create(podTranslated)
	if err != nil {
		return errors.Wrap(err, "Unable to create pod")
	}
	log.G(ctx).Info("Pod", podServer.Name, "successfully created on remote cluster")
	// Here we have to change the view of the remote POD to show it as a local one
	p.notifier(pod)

	p.publishPod(pod)

	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *KubernetesProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	ctx, span := trace.StartSpan(ctx, "UpdatePod")

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Infof("receive UpdatePod %q", pod.Name)

	if pod == nil {
		return errors.New("pod cannot be nil")
	}
	nattedNS := p.NatNamespace(pod.Namespace, false)
	if nattedNS == "" {
		return nil
	}

	podTranslated := H2FTranslate(pod, nattedNS)

	poUpdated, err := p.foreignClient.CoreV1().Pods(nattedNS).Get(podTranslated.Name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Unable to create pod")
	}
	podInverse := F2HTranslate(poUpdated, p.RemappedPodCidr, pod.Namespace)
	p.notifier(podInverse)

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *KubernetesProvider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	ctx, span := trace.StartSpan(ctx, "DeletePod")
	defer span.End()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, pod.Namespace, nameKey, pod.Name)

	log.G(ctx).Infof("receive DeletePod %q", pod.Name)
	opts := &metav1.DeleteOptions{}

	nattedNS := p.NatNamespace(pod.Namespace, false)
	if nattedNS == "" {
		return errors.New("cannot delete pod belonging to a non-natted namespace")
	}

	err = p.foreignClient.CoreV1().Pods(nattedNS).Delete(pod.Name,opts)
	if err != nil {
		return errors.Wrap(err, "Unable to delete pod")
	}

	now := metav1.Now()
	pod.Status.Phase = v1.PodSucceeded
	pod.Status.Reason = "KubernetesProviderPodDeleted"
	//
	for idx := range pod.Status.ContainerStatuses {
		pod.Status.ContainerStatuses[idx].Ready = false
		// We fix to now the starting container when reconciliating a container which is
		if pod.Status.ContainerStatuses[idx].State.Running == nil {
			pod.Status.ContainerStatuses[idx].State.Running = &v1.ContainerStateRunning{StartedAt:now}
		}
		pod.Status.ContainerStatuses[idx].State = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				Message:    "Kubernetes provider terminated container upon deletion",
				FinishedAt: now,
				Reason:     "KubernetesProviderPodContainerDeleted",
				StartedAt:  pod.Status.ContainerStatuses[idx].State.Running.StartedAt,
			},
		}
	}

	p.deletePod(pod)
	p.notifier(pod)

	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *KubernetesProvider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	ctx, span := trace.StartSpan(ctx, "GetPod")
	defer func() {
		span.SetStatus(err)
		span.End()
	}()

	// Add the pod's coordinates to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, name)

	log.G(ctx).Infof("receive GetPod %q", name)
	opts := metav1.GetOptions{}

	nattedNS := p.NatNamespace(namespace, false)

	if nattedNS == "" {
		return nil, nil
	}

	podServer, err := p.foreignClient.CoreV1().Pods(nattedNS).Get(name,opts)
	if err != nil {
		if kerror.IsNotFound(err) {
			return nil, errdefs.NotFoundf("pod \"%s/%s\" is not known to the provider", namespace, name)
		}
		return nil, errors.Wrap(err, "Unable to get pod")
	}

	podInverted := F2HTranslate(podServer, p.RemappedPodCidr, namespace)
	return podInverted, nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *KubernetesProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	ctx, span := trace.StartSpan(ctx, "GetPodStatus")
	defer span.End()

	// Add namespace and name as attributes to the current span.
	ctx = addAttributes(ctx, span, namespaceKey, namespace, nameKey, name)

	nattedNS := p.NatNamespace(namespace, false)

	if nattedNS == "" {
		return nil, nil
	}

	podForeignIn, err := p.foreignClient.CoreV1().Pods(nattedNS).Get(name,metav1.GetOptions{})
    if err != nil {
    	return nil,errors.Wrap(err,"error getting status")
	}
	podOutput := F2HTranslate(podForeignIn,p.RemappedPodCidr, namespace)
	log.G(ctx).Infof("receive GetPodStatus %q", name)
	return &podOutput.Status,nil
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *KubernetesProvider) RunInContainer(ctx context.Context, namespace string, podName string, containerName string, cmd []string, attach api.AttachIO) error {
	req := p.foreignClient.CoreV1().RESTClient().
		Post().
		Namespace(namespace).
		Resource("pods").
		Name(podName).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
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
func (p *KubernetesProvider) GetContainerLogs(ctx context.Context, namespace string, podName string, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	options := &v1.PodLogOptions{
		Container: containerName,
	}
	logs := p.foreignClient.CoreV1().Pods(namespace).GetLogs(podName, options)
	stream, err := logs.Stream()
	if err != nil {
		return nil, fmt.Errorf("could not get stream from logs request: %v", err)
	}
	return stream, nil
}

// GetPods returns a list of all pods known to be "running".
func (p *KubernetesProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	ctx, span := trace.StartSpan(ctx, "GetPods")
	defer span.End()

	log.G(ctx).Info("receive GetPods")

	if p.foreignClient == nil {
		return nil, nil
	}

	var podsHomeOut []*v1.Pod

	for k, v := range p.namespaceNatting {
		podsForeignIn, err := p.foreignClient.CoreV1().Pods(v).List(metav1.ListOptions{})
		if p == nil {
			continue
		}

		if err != nil {
			if kerror.IsNotFound(err) {
				return nil, errdefs.NotFoundf("pods in \"%s\" is not known to the provider", k)
			}
			return nil, errors.Wrap(err, "Unable to get pods")
		}

		for _, pod := range podsForeignIn.Items {
			podsHomeOut = append(podsHomeOut, H2FTranslate(&pod, k))
		}
	}

	return podsHomeOut, nil
}


// GetStatsSummary returns dummy stats for all pods known by this provider.
func (p *KubernetesProvider) GetStatsSummary(ctx context.Context) (*stats.Summary, error) {
	var span trace.Span
	ctx, span = trace.StartSpan(ctx, "GetStatsSummary") //nolint: ineffassign
	defer span.End()

	// Grab the current timestamp so we can report it as the time the stats were generated.
	t := metav1.NewTime(time.Now())

	// Create the Summary object that will later be populated with node and pod stats.
	res := &stats.Summary{}

	// Populate the Summary object with basic node stats.
	res.Node = stats.NodeStats{
		NodeName:  p.nodeName,
		StartTime: metav1.NewTime(p.startTime),
	}

	// Populate the Summary object with dummy stats for each pod known by this provider.
	// TODO: modity the namespace we list the pods from
	pods, err := p.foreignClient.CoreV1().Pods("").List(metav1.ListOptions{})
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

// NotifyPods is called to set a pod notifier callback function. This should be called before any operations are ready
// within the provider.
func (p *KubernetesProvider) NotifyPods(ctx context.Context, notifier func(*v1.Pod)) {
	p.notifier = notifier
}

// addAttributes adds the specified attributes to the provided span.
// attrs must be an even-sized list of string arguments.
// Otherwise, the span won't be modified.
// TODO: Refactor and move to a "tracing utilities" package.
func addAttributes(ctx context.Context, span trace.Span, attrs ...string) context.Context {
	if len(attrs)%2 == 1 {
		return ctx
	}
	for i := 0; i < len(attrs); i += 2 {
		ctx = span.WithField(ctx, attrs[i], attrs[i+1])
	}
	return ctx
}

func (p *KubernetesProvider) CreateNamespaceIfNotExisting(name string) error {

	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := p.foreignClient.CoreV1().Namespaces().Create(ns)
	if err != nil && !kerror.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (p *KubernetesProvider) NatNamespace(name string, create bool) string {
	var ok bool
	var ns string

	if ns, ok = p.namespaceNatting[name]; !ok {
		if create {
			ns = strings.Join([]string{p.homeClusterID, name}, "-")
			p.namespaceNatting[name] = ns
			p.namespaceDeNatting[ns] = name
		}
	}

	return ns
}

func (p *KubernetesProvider) DeNatNamespace(name string) string {
	if ns, ok := p.namespaceDeNatting[name]; !ok {
		return ""
	} else {
		return ns
	}
}
