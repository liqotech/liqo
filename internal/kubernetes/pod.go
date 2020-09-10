package kubernetes

import (
	"context"
	"fmt"
	"github.com/liqotech/liqo/internal/errdefs"
	"github.com/liqotech/liqo/internal/node/api"
	"github.com/liqotech/liqo/internal/trace"
	"github.com/pkg/errors"
	"io"
	v1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	"math/rand"
	"strings"
	"time"
)

const (
	updateTiming = 500 * time.Millisecond
)

// CreatePod accepts a Pod definition and stores it in memory.
func (p *KubernetesProvider) CreatePod(ctx context.Context, pod *v1.Pod) error {
	// Add the pod's coordinates to the current span.
	if pod == nil {
		return errors.New("pod cannot be nil")
	}

	klog.Infof("receive CreatePod %q", pod.Name)

	if pod.OwnerReferences != nil && len(pod.OwnerReferences) != 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
		msg := fmt.Sprintf("Skip to create DaemonSet pod %q", pod.Name)
		klog.Info(msg)
		return nil
	}

	nattedNS, err := p.NatNamespace(pod.Namespace, true)
	if err != nil {
		return err
	}

	podTranslated := H2FTranslate(pod, nattedNS)

	podServer, err := p.foreignClient.Client().CoreV1().Pods(podTranslated.Namespace).Create(context.TODO(), podTranslated, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	klog.Info("Pod", podServer.Name, "successfully created on remote cluster")

	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *KubernetesProvider) UpdatePod(ctx context.Context, pod *v1.Pod) error {
	if pod == nil {
		return errors.New("pod cannot be nil")
	}
	// Add the pod's coordinates to the current span.
	nattedNS, err := p.NatNamespace(pod.Namespace, false)
	if err != nil {
		return err
	}

	podTranslated := H2FTranslate(pod, nattedNS)

	if p.foreignPodCaches == nil || p.foreignPodCaches[nattedNS] == nil {
		_, err = p.foreignClient.Client().CoreV1().Pods(nattedNS).Get(context.TODO(), podTranslated.Name, metav1.GetOptions{})
	} else {
		_, err = p.foreignPodCaches[nattedNS].get(podTranslated.Name, metav1.GetOptions{})
	}
	if err != nil {
		klog.Warningf("Cannot update pod \"%s\" in provider namespace \"%s\"", podTranslated.Name, podTranslated.Namespace)
		return err
	}
	klog.V(3).Infof("Updated pod \"%s\" in provider namespace \"%s\"", podTranslated.Name, podTranslated.Namespace)

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *KubernetesProvider) DeletePod(ctx context.Context, pod *v1.Pod) (err error) {
	klog.Infof("receive DeletePod %q", pod.Name)
	opts := &metav1.DeleteOptions{}

	nattedNS, err := p.NatNamespace(pod.Namespace, false)
	if err != nil {
		return err
	}

	err = p.foreignClient.Client().CoreV1().Pods(nattedNS).Delete(context.TODO(), pod.Name, *opts)
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
			pod.Status.ContainerStatuses[idx].State.Running = &v1.ContainerStateRunning{StartedAt: now}
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

	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *KubernetesProvider) GetPod(ctx context.Context, namespace, name string) (pod *v1.Pod, err error) {
	klog.V(3).Infof("receive GetPod %q", name)

	nattedNS, err := p.NatNamespace(namespace, false)
	if err != nil {
		return nil, err
	}

	var podServer *v1.Pod
	if p.foreignPodCaches == nil || p.foreignPodCaches[nattedNS] == nil {
		podServer, err = p.foreignClient.Client().CoreV1().Pods(nattedNS).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		podServer, err = p.foreignPodCaches[nattedNS].get(name, metav1.GetOptions{})
	}
	if err != nil {
		if kerror.IsNotFound(err) {
			return nil, errdefs.NotFoundf("pod \"%s/%s\" is not known to the provider", namespace, name)
		}
		return nil, errors.Wrap(err, "Unable to get pod")
	}

	podInverted := F2HTranslate(podServer, p.RemoteRemappedPodCidr, namespace)
	return podInverted, nil
}

// GetPodStatus returns the status of a pod by name that is "running".
// returns nil if a pod by that name is not found.
func (p *KubernetesProvider) GetPodStatus(ctx context.Context, namespace, name string) (*v1.PodStatus, error) {
	nattedNS, err := p.NatNamespace(namespace, false)

	if err != nil {
		return nil, nil
	}

	var podForeignIn *v1.Pod
	if p.foreignPodCaches == nil || p.foreignPodCaches[nattedNS] == nil {
		podForeignIn, err = p.foreignClient.Client().CoreV1().Pods(nattedNS).Get(context.TODO(), name, metav1.GetOptions{})
	} else {
		podForeignIn, err = p.foreignPodCaches[nattedNS].get(name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, errors.Wrap(err, "error getting status")
	}
	podOutput := F2HTranslate(podForeignIn, p.RemoteRemappedPodCidr, namespace)
	klog.Infof("receive GetPodStatus %q", name)
	return &podOutput.Status, nil
}

// RunInContainer executes a command in a container in the pod, copying data
// between in/out/err and the container's stdin/stdout/stderr.
func (p *KubernetesProvider) RunInContainer(ctx context.Context, namespace string, podName string, containerName string, cmd []string, attach api.AttachIO) error {

	nattedNS, err := p.NatNamespace(namespace, false)
	if err != nil {
		return err
	}

	req := p.foreignClient.Client().CoreV1().RESTClient().
		Post().
		Namespace(nattedNS).
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
	nattedNS, err := p.NatNamespace(namespace, false)
	if err != nil {
		return nil, err
	}

	options := &v1.PodLogOptions{
		Container: containerName,
	}
	logs := p.foreignClient.Client().CoreV1().Pods(nattedNS).GetLogs(podName, options)
	stream, err := logs.Stream(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("could not get stream from logs request: %v", err)
	}
	return stream, nil
}

// GetPods returns a list of all pods known to be "running".
func (p *KubernetesProvider) GetPods(ctx context.Context) ([]*v1.Pod, error) {
	klog.Info("receive GetPods")

	if p.foreignClient == nil {
		return nil, nil
	}

	var podsHomeOut []*v1.Pod

	nt, err := p.ntCache.getNattingTable(p.foreignClusterId)
	if err != nil {
		return nil, err
	}

	if nt == nil || nt.Spec.NattingTable == nil {
		klog.Info("empty natting table")
		return podsHomeOut, nil
	}

	for k, v := range nt.Spec.NattingTable {
		var podsForeignIn *v1.PodList
		var err error

		if p.foreignPodCaches == nil || p.foreignPodCaches[v] == nil {
			podsForeignIn, err = p.foreignClient.Client().CoreV1().Pods(v).List(context.TODO(), metav1.ListOptions{})
		} else {
			podsForeignIn, err = p.foreignPodCaches[v].list(metav1.ListOptions{})
		}
		if err != nil {
			if kerror.IsNotFound(err) {
				return nil, errdefs.NotFoundf("pods in \"%s\" is not known to the provider", k)
			}
			return nil, errors.Wrap(err, "Unable to get pods")
		}

		for i := range podsForeignIn.Items {
			pod := podsForeignIn.Items[i]
			podsHomeOut = append(podsHomeOut, H2FTranslate(&pod, k))
		}
	}

	return podsHomeOut, nil
}

// GetStatsSummary returns dummy stats for all pods known by this provider.
func (p *KubernetesProvider) GetStatsSummary(ctx context.Context) (*stats.Summary, error) {
	var span trace.Span
	_, span = trace.StartSpan(ctx, "GetStatsSummary") //nolint: ineffassign
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
	pods, err := p.foreignClient.Client().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
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

type podCache struct {
	client    kubernetes.Interface
	namespace string
	store     cache.Store
	stop      chan struct{}
}

// newForeignPodCache creates a new cache that serves the remote pods for a specific endpoint.
func newForeignPodCache(c kubernetes.Interface, namespace string) *podCache {
	listFunc := func(ls metav1.ListOptions) (result runtime.Object, err error) {
		return c.CoreV1().Pods(namespace).List(context.TODO(), ls)
	}
	watchFunc := func(ls metav1.ListOptions) (watch.Interface, error) {
		return c.CoreV1().Pods(namespace).Watch(context.TODO(), ls)
	}

	store, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		&v1.Pod{},
		1*time.Second,
		cache.ResourceEventHandlerFuncs{},
	)

	stopChan := make(chan struct{}, 1)
	go controller.Run(stopChan)

	return &podCache{
		client:    c,
		store:     store,
		stop:      stopChan,
		namespace: namespace,
	}
}

func (c *podCache) get(name string, options metav1.GetOptions) (*v1.Pod, error) {
	n := strings.Join([]string{c.namespace, name}, "/")
	po, found, _ := c.store.GetByKey(n)
	if found {
		klog.V(6).Infof("pod %v fetched from cache", name)
		return po.(*v1.Pod), nil
	}

	po, err := c.client.CoreV1().Pods(c.namespace).Get(context.TODO(), name, options)
	if err != nil {
		return nil, err
	}

	klog.V(6).Infof("pod %v fetched from remote", name)
	return po.(*v1.Pod), nil
}

func (c *podCache) list(options metav1.ListOptions) (*v1.PodList, error) {
	podList := &v1.PodList{
		Items: []v1.Pod{},
	}
	if c.store == nil {
		klog.V(6).Info("pods listed from remote")
		return c.client.CoreV1().Pods(c.namespace).List(context.TODO(), options)
	}

	pods := c.store.List()
	if pods == nil {
		klog.V(6).Info("pods listed from remote")
		return c.client.CoreV1().Pods(c.namespace).List(context.TODO(), options)
	}

	for _, po := range pods {
		podList.Items = append(podList.Items, po.(v1.Pod))
	}

	klog.V(6).Info("pods listed from cache")
	return podList, nil
}

type podEventCounter struct {
	counter int
	po      *v1.Pod
}

// watchForeignPods watch the remote pod transitions for a specific namespace
// each transition should trigger a local update, through the p.notifier methods.
// In order to avoid throttling, this notification mechanism is timed by
// a ticker component
func (p *KubernetesProvider) watchForeignPods(watcher watch.Interface, stop chan struct{}) {
	pods := make(map[string]podEventCounter)
	ticker := time.NewTicker(updateTiming)

	for {
		select {
		case <-stop:
			watcher.Stop()
			p.powg.Done()
			return
		case <-ticker.C:
			klog.V(5).Info("ticker triggered home pods reflection")
			po := getMostImportantPod(pods)
			if po == nil {
				break
			}
			poUpdate := podEventCounter{
				counter: 0,
				po:      po,
			}
			pods[po.Name] = poUpdate
			denattedNS, err := p.DeNatNamespace(po.Namespace)
			if err != nil {
				klog.Error(err, "natting error in watchForeignPods")
			}
			p.notifier(F2HTranslate(po, p.RemoteRemappedPodCidr, denattedNS))
		case e := <-watcher.ResultChan():
			po, ok := e.Object.(*v1.Pod)
			if !ok {
				klog.Error("unexpected type")
				break
			}
			klog.V(6).Infof("new event %v for pod %v", e.Type, po.Name)
			if _, ok := pods[po.Name]; !ok {
				pods[po.Name] = podEventCounter{
					counter: 1,
					po:      po,
				}
			} else {
				poUpdate := podEventCounter{
					counter: pods[po.Name].counter + 1,
					po:      po,
				}
				pods[po.Name] = poUpdate
			}
		}
	}
}

// dummy function needed to update only one pod at a time. This implementation
// can lead to a (very unlikely) starvation for a specific pod. It should be replaced by a smarter algorithm.
func getMostImportantPod(pods map[string]podEventCounter) *v1.Pod {
	var bestK string
	var bestCounter int
	for k, pc := range pods {
		if pc.counter > bestCounter {
			bestCounter = pc.counter
			bestK = k
		}
	}
	if bestCounter == 0 {
		return nil
	}

	return pods[bestK].po
}
