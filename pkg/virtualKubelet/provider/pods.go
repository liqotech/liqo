// Copyright 2019-2021 The Liqo Authors
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

package provider

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/modern-go/reflect2"
	"github.com/pkg/errors"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	stats "github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	remotecommandclient "k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmgt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	vkContext "github.com/liqotech/liqo/pkg/virtualKubelet/context"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

// CreatePod accepts a Pod definition and stores it in memory.
func (p *LiqoProvider) CreatePod(ctx context.Context, homePod *corev1.Pod) error {
	if reflect2.IsNil(homePod) {
		klog.V(4).Info("received nil pod to create")
		return nil
	}

	klog.V(3).Infof("PROVIDER: pod %s/%s asked to be created in the provider", homePod.Namespace, homePod.Name)

	if homePod.OwnerReferences != nil && len(homePod.OwnerReferences) != 0 && homePod.OwnerReferences[0].Kind == "DaemonSet" {
		klog.Infof("PROVIDER: Skip to create DaemonSet homePod %q", homePod.Name)
		return nil
	}

	foreignObj, err := forge.HomeToForeign(homePod, nil, forge.LiqoOutgoingKey)
	if err != nil {
		klog.V(4).Infof("PROVIDER: error while forging remote pod %s/%s because of error %v", homePod.Namespace, homePod.Name, err)
		return kerror.NewServiceUnavailable(err.Error())
	}
	foreignPod := foreignObj.(*corev1.Pod)

	foreignReplicaset := forge.ReplicasetFromPod(foreignPod)

	// Add a finalizer to allow the pod to be garbage collected by the incoming replicaset reflector.
	// Add label to distinct the offloaded pods from the local ones.
	// The merge strategy is types.StrategicMergePatchType in order to merger the previous state
	// with the new configuration.
	homePodPatch := []byte(fmt.Sprintf(
		`{"metadata":{"labels":{"%s":"%s"},"finalizers":["%s"]}}`,
		liqoconst.LocalPodLabelKey, liqoconst.LocalPodLabelValue, virtualKubelet.HomePodFinalizer))

	_, err = p.homeClient.CoreV1().Pods(homePod.Namespace).Patch(context.TODO(),
		homePod.Name,
		types.StrategicMergePatchType,
		homePodPatch,
		metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return kerror.NewServiceUnavailable(err.Error())
	}

	_, err = p.foreignClient.AppsV1().ReplicaSets(foreignReplicaset.Namespace).Create(context.TODO(), foreignReplicaset, metav1.CreateOptions{})
	if kerror.IsAlreadyExists(err) {
		klog.V(4).Infof("PROVIDER: creation of foreign replicaset %s/%s aborted, already existing", foreignReplicaset.Namespace, foreignReplicaset.Name)
		return nil
	}
	if err != nil {
		klog.Error(err)
		return kerror.NewServiceUnavailable(err.Error())
	}

	klog.V(3).Infof("PROVIDER: replicaset %v/%v successfully created on remote cluster", foreignReplicaset.Namespace, foreignReplicaset.Name)

	return nil
}

// UpdatePod accepts a Pod definition and updates its reference.
func (p *LiqoProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	if reflect2.IsNil(pod) {
		klog.V(4).Info("received nil pod to create")
		return nil
	}

	klog.V(3).Infof("PROVIDER: pod %s/%s asked to be updated in the provider", pod.Namespace, pod.Name)

	return nil
}

// DeletePod deletes the specified pod out of memory.
func (p *LiqoProvider) DeletePod(ctx context.Context, pod *corev1.Pod) (err error) {
	if reflect2.IsNil(pod) {
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
			klog.V(3).Infof("PROVIDER: home pod %s/%s foreign replica not deleted because unlabeled", pod.Namespace, pod.Name)
			return nil
		}
	} else {
		replicasetName = pod.Name
		foreignNamespace, err = p.namespaceMapper.NatNamespace(pod.Namespace)
		if err != nil {
			return nil
		}
	}

	err = p.foreignClient.AppsV1().ReplicaSets(foreignNamespace).Delete(context.TODO(), replicasetName, metav1.DeleteOptions{})
	if kerror.IsNotFound(err) {
		klog.V(5).Infof("PROVIDER: replicaset %v/%v not deleted because not existing", foreignNamespace, replicasetName)
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "Unable to delete foreign replicaset")
	}

	klog.V(3).Infof("PROVIDER: replicaset %v/%v successfully deleted on remote cluster", foreignNamespace, pod.Name)

	return nil
}

// GetPod returns a pod by name that is stored in memory.
func (p *LiqoProvider) GetPod(ctx context.Context, namespace, name string) (pod *corev1.Pod, err error) {
	if reflect2.IsNil(pod) {
		klog.V(4).Info("PROVIDER: received nil pod")
		return nil, nil
	}

	klog.V(3).Infof("PROVIDER: pod %s/%s requested to the provider", namespace, name)

	foreignNamespace, err := p.namespaceMapper.NatNamespace(namespace)
	if err != nil {
		klog.V(4).Infof("PROVIDER: cannot get remote pod %s/%s because of error %v, requeueing", pod.Namespace, pod.Name, err)
		return nil, nil
	}

	_, err = p.apiController.CacheManager().GetForeignAPIByIndex(apimgmgt.Pods, foreignNamespace, name)
	if err != nil {
		klog.V(4).Infof("PROVIDER: cannot get remote pod %s/%s because of error %v, requeueing", pod.Namespace, pod.Name, err)
		return nil, nil
	}

	homePod, err := p.apiController.CacheManager().GetHomeNamespacedObject(apimgmgt.Pods, namespace, name)
	if err != nil {
		klog.V(4).Infof("PROVIDER: cannot get remote pod %s/%s because of error %v, requeueing", pod.Namespace, pod.Name, err)
		return nil, nil
	}

	// if we want to enforce some foreign pod fields we should return a homePod having the fields to enforce
	// taken from the foreign pod
	return homePod.(*corev1.Pod), nil
}

// GetPodStatus is currently not implemented, panic if the method gets invoked.
// GetPodStatus should only be called by the virtual kubelet if the provider does not implement the PodNotifier interface.
// The LiqoProvider implements PodNotifier interface so we don't expect GetPodStatus to get called.
func (p *LiqoProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	panic("Virtual Kubelet called GetPodStatus unexpectedly.")
}

// GetPods returns a list of all pods known to be "running".
func (p *LiqoProvider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	klog.V(3).Infof("PROVIDER: foreign pod listing requested to the provider")

	var homePods []*corev1.Pod

	for foreignNamespace := range p.namespaceMapper.MappedNamespaces() {
		pods, err := p.apiController.CacheManager().ListForeignNamespacedObject(apimgmgt.Pods, foreignNamespace)
		if err != nil {
			return nil, errors.New("Unable to get pods")
		}

		for _, pod := range pods {
			homePod, err := forge.ForeignToHome(pod.(*corev1.Pod), nil, forge.LiqoNodeName())
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
func (p *LiqoProvider) RunInContainer(ctx context.Context, homeNamespace, homePodName, containerName string,
	cmd []string, attach api.AttachIO) error {
	foreignNamespace, err := p.namespaceMapper.NatNamespace(homeNamespace)
	if err != nil {
		return err
	}

	foreignObj, err := p.apiController.CacheManager().GetForeignAPIByIndex(apimgmgt.Pods, foreignNamespace, homePodName)
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

	exec, err := remotecommandclient.NewSPDYExecutor(p.foreignRestConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("could not make remote command: %v", err)
	}

	err = exec.Stream(remotecommandclient.StreamOptions{
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
func (p *LiqoProvider) GetContainerLogs(ctx context.Context, homeNamespace, homePodName, containerName string,
	opts api.ContainerLogOpts) (io.ReadCloser, error) {
	foreignNamespace, err := p.namespaceMapper.NatNamespace(homeNamespace)
	if err != nil {
		return nil, err
	}

	foreignObj, err := p.apiController.CacheManager().GetForeignAPIByIndex(apimgmgt.Pods, foreignNamespace, homePodName)
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
		optsSinceSeconds := int64(opts.SinceSeconds)
		logOptions.SinceSeconds = &optsSinceSeconds
	}
	if !opts.SinceTime.IsZero() {
		optsSinceTime := metav1.NewTime(opts.SinceTime)
		logOptions.SinceTime = &optsSinceTime
	}
	if opts.LimitBytes > 0 {
		optsLimitBytes := int64(opts.LimitBytes)
		logOptions.LimitBytes = &optsLimitBytes
	}
	if opts.Tail > 0 {
		optsTail := int64(opts.Tail)
		logOptions.TailLines = &optsTail
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
	// Grab the current timestamp so we can report it as the time the stats were generated.
	t := metav1.NewTime(time.Now())

	// Create the Summary object that will later be populated with node and pod stats.
	res := &stats.Summary{}

	// Populate the Summary object with basic node stats.
	res.Node = stats.NodeStats{
		NodeName:  p.nodeName.Value().ToString(),
		StartTime: metav1.NewTime(p.startTime),
	}

	var (
		// nodeTotalNanoCore will be populated with the sum of the values of UsageNanoCores computes across all
		// containers in all the pods running in the remote cluster on behalf of this virtual node.
		nodeTotalNanoCore uint64
		// nodeTotalNanoBytes will be populated with the sum of the values of UsageBytes computed across all containers
		// in all the pods running in the remote cluster on behalf of this virtual node.
		nodeTotalNanoBytes uint64
	)

	// iterates over all the mapped namespaces
	for home, foreign := range p.namespaceMapper.MappedNamespaces() {
		// get the metrics of the foreign pods in each namespace by filtering with the liqoOutgoingKey
		podMetrics, err := p.foreignMetricsClient.MetricsV1beta1().PodMetricses(foreign).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", forge.LiqoOutgoingKey, forge.LiqoNodeName()),
		})
		if err != nil {
			return nil, errors.Wrapf(err, "error while listing foreign pod metricses in namespace %s", foreign)
		}

		for index := range podMetrics.Items {
			// fetch foreign pod from cache
			foreignObj, err := p.apiController.CacheManager().GetForeignNamespacedObject(apimgmgt.Pods, foreign, podMetrics.Items[index].Name)
			if err != nil {
				return nil, errors.Errorf("error while retrieving foreign pod %s/%s from cache", foreign, podMetrics.Items[index].Name)
			}
			// retrieve foreign pod name by the foreign pod label
			foreignPod := foreignObj.(*corev1.Pod)
			if foreignPod.Labels == nil {
				return nil, errors.Errorf("error in foreign pod, empty %s label set in pod %s/%s",
					virtualKubelet.ReflectedpodKey, foreign, podMetrics.Items[index].Name)
			}
			homePodName, ok := foreignPod.Labels[virtualKubelet.ReflectedpodKey]
			if !ok {
				return nil, errors.Errorf("error in foreign pod, missing %s label in pod %s/%s",
					virtualKubelet.ReflectedpodKey, foreign, podMetrics.Items[index].Name)
			}

			// fetch home pod from cache
			homeObj, err := p.apiController.CacheManager().GetHomeNamespacedObject(apimgmgt.Pods, home, homePodName)
			if err != nil {
				return nil, errors.Errorf("error while retrieving home pod %s/%s from cache", home, homePodName)
			}
			homePod := homeObj.(*corev1.Pod)

			// Create a PodStats object to populate with pod stats.
			podStats := stats.PodStats{
				PodRef: stats.PodReference{
					Name:      homePodName,
					Namespace: home,
					UID:       string(homePod.UID),
				},
				StartTime: homePod.CreationTimestamp,
			}

			var (
				// totalUsageNanoCores will be populated with the sum of the values of UsageNanoCores computes across all containers in the pod.
				totalUsageNanoCores uint64
				// totalUsageBytes will be populated with the sum of the values of UsageBytes computed across all containers in the pod.
				totalUsageBytes uint64
			)

			// Iterate over all containers in the current pod to get stats
			for _, container := range podMetrics.Items[index].Containers {
				nanoCpuUsage := uint64(container.Usage.Cpu().ScaledValue(resource.Nano))
				totalUsageNanoCores += nanoCpuUsage

				nanoMemoryUsage := uint64(container.Usage.Memory().Value())
				totalUsageBytes += nanoMemoryUsage

				// Append a ContainerStats object containing the dummy stats to the PodStats object.
				podStats.Containers = append(podStats.Containers, stats.ContainerStats{
					Name:      container.Name,
					StartTime: homePod.CreationTimestamp,
					CPU: &stats.CPUStats{
						Time:           t,
						UsageNanoCores: &nanoCpuUsage,
					},
					Memory: &stats.MemoryStats{
						Time:            t,
						UsageBytes:      &nanoMemoryUsage,
						WorkingSetBytes: &nanoMemoryUsage,
					},
				})
			}

			nodeTotalNanoCore += totalUsageNanoCores
			nodeTotalNanoBytes += totalUsageBytes

			// Populate the CPU and RAM stats for the pod and append the PodsStats object to the Summary object to be returned.
			podStats.CPU = &stats.CPUStats{
				Time:           t,
				UsageNanoCores: &totalUsageNanoCores,
			}
			podStats.Memory = &stats.MemoryStats{
				Time:            t,
				UsageBytes:      &totalUsageBytes,
				WorkingSetBytes: &totalUsageBytes,
			}
			res.Pods = append(res.Pods, podStats)
		}
	}

	res.Node.CPU = &stats.CPUStats{
		Time:           t,
		UsageNanoCores: &nodeTotalNanoCore,
	}
	res.Node.Memory = &stats.MemoryStats{
		Time:            t,
		UsageBytes:      &nodeTotalNanoBytes,
		WorkingSetBytes: &nodeTotalNanoBytes,
	}

	return res, nil
}

// NotifyPods is called to set a pod informing callback function. This should be called before any operations are ready
// within the provider.
func (p *LiqoProvider) NotifyPods(ctx context.Context, notifier func(*corev1.Pod)) {
	p.apiController.SetInformingFunc(apimgmgt.Pods, notifier)
	p.apiController.SetInformingFunc(apimgmgt.ReplicaSets, notifier)
}
