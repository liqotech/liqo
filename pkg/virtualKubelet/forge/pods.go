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

package forge

import (
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	// PodOffloadingBackoffReason -> the reason assigned to pods rejected by the virtual kubelet before offloading has started.
	PodOffloadingBackoffReason = "OffloadingBackoff"
	// PodOffloadingAbortedReason -> the reason assigned to pods rejected by the virtual kubelet after offloading has started.
	PodOffloadingAbortedReason = "OffloadingAborted"
)

// PodIPTranslator defines the function to translate between remote and local IP addresses.
type PodIPTranslator func(string) string

// LocalPod forges the object meta and status of the local pod, given the remote one.
func LocalPod(local, remote *corev1.Pod, translator PodIPTranslator, restarts int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: *local.ObjectMeta.DeepCopy(),
		Status:     LocalPodStatus(remote.Status.DeepCopy(), translator, restarts),
	}
}

// LocalPodStatus forges the status of the local pod, given the remote one.
func LocalPodStatus(remote *corev1.PodStatus, translator PodIPTranslator, restarts int32) corev1.PodStatus {
	// Translate the relevant IPs
	if remote.PodIP != "" {
		remote.PodIP = translator(remote.PodIP)
		remote.PodIPs = []corev1.PodIP{{IP: remote.PodIP}}
	}
	remote.HostIP = LiqoNodeIP

	// Increase the restart count if necessary
	for idx := range remote.ContainerStatuses {
		remote.ContainerStatuses[idx].RestartCount += restarts
	}

	return *remote
}

// LocalRejectedPod forges the status of a local rejected pod.
func LocalRejectedPod(local *corev1.Pod, phase corev1.PodPhase, reason string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: *local.ObjectMeta.DeepCopy(),
		Status:     LocalRejectedPodStatus(local.Status.DeepCopy(), phase, reason),
	}
}

// LocalRejectedPodStatus forges the status of the local rejected pod.
func LocalRejectedPodStatus(local *corev1.PodStatus, phase corev1.PodPhase, reason string) corev1.PodStatus {
	local.Phase = phase
	local.Reason = reason
	return *local
}

// RemoteShadowPod forges the reflected shadowpod, given the local one.
func RemoteShadowPod(local *corev1.Pod, remote *vkv1alpha1.ShadowPod, targetNamespace string) *vkv1alpha1.ShadowPod {
	if remote == nil {
		// The remote is nil if not already created.
		remote = &vkv1alpha1.ShadowPod{ObjectMeta: metav1.ObjectMeta{Name: local.GetName(), Namespace: targetNamespace}}
	}

	return &vkv1alpha1.ShadowPod{
		ObjectMeta: RemoteObjectMeta(&local.ObjectMeta, &remote.ObjectMeta),
		Spec: vkv1alpha1.ShadowPodSpec{
			Pod: RemotePodSpec(local.Spec.DeepCopy(), remote.Spec.Pod.DeepCopy()),
		},
	}
}

// RemotePodSpec forges the specs of the reflected pod specs, given the local ones.
// It expects the local and remote objects to be deepcopies, as they are mutated.
func RemotePodSpec(local, remote *corev1.PodSpec) corev1.PodSpec {
	remote.TerminationGracePeriodSeconds = local.TerminationGracePeriodSeconds
	remote.Volumes = forgeVolumes(local.Volumes)
	remote.InitContainers = forgeContainers(local.InitContainers, remote.Volumes)
	remote.Containers = forgeContainers(local.Containers, remote.Volumes)
	remote.Tolerations = RemoteTolerations(local.Tolerations)
	remote.EnableServiceLinks = local.EnableServiceLinks
	remote.Hostname = local.Hostname
	remote.Subdomain = local.Subdomain
	remote.TopologySpreadConstraints = local.TopologySpreadConstraints
	remote.RestartPolicy = local.RestartPolicy
	remote.AutomountServiceAccountToken = local.AutomountServiceAccountToken
	remote.SecurityContext = local.SecurityContext

	return *remote
}

// RemoteTolerations forges the tolerations for a reflected pod.
func RemoteTolerations(inputTolerations []corev1.Toleration) []corev1.Toleration {
	tolerations := make([]corev1.Toleration, 0)

	for _, toleration := range inputTolerations {
		// copy all tolerations except the one for the virtual node.
		// This prevents by default the possibility of "recursive" scheduling on virtual nodes on the target cluster.
		if toleration.Key != liqoconst.VirtualNodeTolerationKey {
			tolerations = append(tolerations, toleration)
		}
	}

	return tolerations
}

func forgeContainers(inputContainers []corev1.Container, inputVolumes []corev1.Volume) []corev1.Container {
	containers := make([]corev1.Container, 0)

	for _, container := range inputContainers {
		volumeMounts := filterVolumeMounts(inputVolumes, container.VolumeMounts)
		containers = append(containers, translateContainer(container, volumeMounts))
	}

	return containers
}

func translateContainer(container corev1.Container, volumes []corev1.VolumeMount) corev1.Container {
	return corev1.Container{
		Name:            container.Name,
		Image:           container.Image,
		Command:         container.Command,
		Args:            container.Args,
		WorkingDir:      container.WorkingDir,
		Ports:           container.Ports,
		Env:             container.Env,
		Resources:       container.Resources,
		LivenessProbe:   container.LivenessProbe,
		ReadinessProbe:  container.ReadinessProbe,
		StartupProbe:    container.StartupProbe,
		SecurityContext: container.SecurityContext,
		VolumeMounts:    volumes,
	}
}

func forgeVolumes(volumesIn []corev1.Volume) []corev1.Volume {
	volumesOut := make([]corev1.Volume, 0)
	for _, v := range volumesIn {
		if v.ConfigMap != nil || v.EmptyDir != nil || v.DownwardAPI != nil || v.Projected != nil || v.PersistentVolumeClaim != nil {
			volumesOut = append(volumesOut, v)
		}
		// copy all volumes of type Secret except for the default token
		if v.Secret != nil && !strings.Contains(v.Secret.SecretName, "default-token") {
			volumesOut = append(volumesOut, v)
		}
	}
	return volumesOut
}

// remove from volumeMountsIn all the volumeMounts with name not contained in volumes.
func filterVolumeMounts(volumes []corev1.Volume, volumeMountsIn []corev1.VolumeMount) []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)
	for _, vm := range volumeMountsIn {
		for _, v := range volumes {
			if vm.Name == v.Name {
				volumeMounts = append(volumeMounts, vm)
			}
		}
	}
	return volumeMounts
}

// LocalNodeStats forges the summary stats for the node managed by the virtual kubelet.
func LocalNodeStats(pods []statsv1alpha1.PodStats) *statsv1alpha1.Summary {
	now := metav1.Now()

	return &statsv1alpha1.Summary{
		Node: statsv1alpha1.NodeStats{
			NodeName: LiqoNodeName, StartTime: metav1.NewTime(StartTime),
			CPU: &statsv1alpha1.CPUStats{
				Time:           now,
				UsageNanoCores: SumPodStats(pods, func(s statsv1alpha1.PodStats) uint64 { return *s.CPU.UsageNanoCores }),
			},
			Memory: &statsv1alpha1.MemoryStats{
				Time:            now,
				UsageBytes:      SumPodStats(pods, func(s statsv1alpha1.PodStats) uint64 { return *s.Memory.UsageBytes }),
				WorkingSetBytes: SumPodStats(pods, func(s statsv1alpha1.PodStats) uint64 { return *s.Memory.WorkingSetBytes }),
			},
		},
		Pods: pods,
	}
}

// LocalPodStats forges the metric stats for a local pod managed by the virtual kubelet.
func LocalPodStats(pod *corev1.Pod, metrics *metricsv1beta1.PodMetrics) statsv1alpha1.PodStats {
	now := metav1.Now()
	containers := LocalContainersStats(metrics.Containers, pod.GetCreationTimestamp(), now)

	return statsv1alpha1.PodStats{
		PodRef: statsv1alpha1.PodReference{
			Name:      pod.GetName(),
			Namespace: pod.GetNamespace(),
			UID:       string(pod.GetUID()),
		},
		StartTime:  pod.GetCreationTimestamp(),
		Containers: containers,
		CPU: &statsv1alpha1.CPUStats{
			Time:           now,
			UsageNanoCores: SumContainerStats(containers, func(s statsv1alpha1.ContainerStats) uint64 { return *s.CPU.UsageNanoCores }),
		},
		Memory: &statsv1alpha1.MemoryStats{
			Time:            now,
			UsageBytes:      SumContainerStats(containers, func(s statsv1alpha1.ContainerStats) uint64 { return *s.Memory.UsageBytes }),
			WorkingSetBytes: SumContainerStats(containers, func(s statsv1alpha1.ContainerStats) uint64 { return *s.Memory.WorkingSetBytes }),
		},
	}
}

// LocalContainersStats forges the metric stats for the containers of a local pod.
func LocalContainersStats(metrics []metricsv1beta1.ContainerMetrics, start, now metav1.Time) []statsv1alpha1.ContainerStats {
	var stats []statsv1alpha1.ContainerStats

	for idx := range metrics {
		stats = append(stats, LocalContainerStats(&metrics[idx], start, now))
	}

	return stats
}

// LocalContainerStats forges the metric stats for a container of a local pod.
func LocalContainerStats(metrics *metricsv1beta1.ContainerMetrics, start, now metav1.Time) statsv1alpha1.ContainerStats {
	Uint64Ptr := func(value uint64) *uint64 { return &value }

	return statsv1alpha1.ContainerStats{
		Name:      metrics.Name,
		StartTime: start,
		CPU: &statsv1alpha1.CPUStats{
			Time:           now,
			UsageNanoCores: Uint64Ptr(uint64(metrics.Usage.Cpu().ScaledValue(resource.Nano))),
		},
		Memory: &statsv1alpha1.MemoryStats{
			Time:            now,
			UsageBytes:      Uint64Ptr(uint64(metrics.Usage.Memory().Value())),
			WorkingSetBytes: Uint64Ptr(uint64(metrics.Usage.Memory().Value())),
		},
	}
}

// SumPodStats returns the sum of the pod stats, given a metric retriever.
func SumPodStats(stats []statsv1alpha1.PodStats, retriever func(statsv1alpha1.PodStats) uint64) *uint64 {
	var sum uint64
	for idx := range stats {
		sum += retriever(stats[idx])
	}
	return &sum
}

// SumContainerStats returns the sum of the container stats, given a metric retriever.
func SumContainerStats(stats []statsv1alpha1.ContainerStats, retriever func(statsv1alpha1.ContainerStats) uint64) *uint64 {
	var sum uint64
	for idx := range stats {
		sum += retriever(stats[idx])
	}
	return &sum
}
