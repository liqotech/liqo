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

package forge

import (
	"fmt"
	"net"
	"strings"

	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/pointer"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	// PodOffloadingBackOffReason -> the reason assigned to pods rejected by the virtual kubelet before offloading has started.
	PodOffloadingBackOffReason = "OffloadingBackOff"
	// PodOffloadingAbortedReason -> the reason assigned to pods rejected by the virtual kubelet after offloading has started.
	PodOffloadingAbortedReason = "OffloadingAborted"

	// ServiceAccountVolumeName is the prefix name that will be added to volumes that mount ServiceAccount secrets.
	// This constant is taken from kubernetes/kubernetes (plugin/pkg/admission/serviceaccount/admission.go).
	ServiceAccountVolumeName = "kube-api-access-"

	// kubernetesAPIService is the DNS name associated with the service targeting the Kubernetes API.
	kubernetesAPIService = "kubernetes.default"
)

// PodIPTranslator defines the function to translate between remote and local IP addresses.
type PodIPTranslator func(string) string

// SASecretRetriever defines the function to retrieve the secret associated with a given service account.
type SASecretRetriever func(string) string

// KubernetesServiceIPGetter defines the function to get the remapped IP associated with the local kubernetes.default service.
type KubernetesServiceIPGetter func() string

// LocalPod forges the object meta and status of the local pod, given the remote one.
func LocalPod(local, remote *corev1.Pod, translator PodIPTranslator, restarts int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: *local.ObjectMeta.DeepCopy(),
		Status:     LocalPodStatus(remote.Status.DeepCopy(), translator, restarts),
	}
}

// LocalPodOffloadedLabel forges the apply patch to add the appropriate label to the offloaded pod.
func LocalPodOffloadedLabel(local *corev1.Pod) (*corev1apply.PodApplyConfiguration, bool) {
	if value, found := local.Labels[liqoconst.LocalPodLabelKey]; found && value == liqoconst.LocalPodLabelValue {
		return nil, false
	}

	return corev1apply.Pod(local.GetName(), local.GetNamespace()).
		WithLabels(map[string]string{liqoconst.LocalPodLabelKey: liqoconst.LocalPodLabelValue}), true
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

	for i := range local.Conditions {
		if local.Conditions[i].Status == corev1.ConditionTrue {
			local.Conditions[i].Status = corev1.ConditionFalse
			local.Conditions[i].Reason = reason
			local.Conditions[i].LastTransitionTime = metav1.Now()
		}
	}

	for i := range local.ContainerStatuses {
		local.ContainerStatuses[i].Ready = false
	}

	return *local
}

// RemoteShadowPod forges the reflected shadowpod, given the local one.
func RemoteShadowPod(local *corev1.Pod, remote *vkv1alpha1.ShadowPod, targetNamespace string,
	saSecretRetriever SASecretRetriever, kubernetesServiceIPRetriever KubernetesServiceIPGetter) *vkv1alpha1.ShadowPod {
	if remote == nil {
		// The remote is nil if not already created.
		remote = &vkv1alpha1.ShadowPod{ObjectMeta: metav1.ObjectMeta{Name: local.GetName(), Namespace: targetNamespace}}
	}

	// Remove the label which identifies offloaded pods, as meaningful only locally.
	FilterLocalPodOffloadedLabel := func(meta *metav1.ObjectMeta) *metav1.ObjectMeta {
		output := meta.DeepCopy()
		delete(output.GetLabels(), liqoconst.LocalPodLabelKey)
		return output
	}

	return &vkv1alpha1.ShadowPod{
		ObjectMeta: RemoteObjectMeta(FilterLocalPodOffloadedLabel(&local.ObjectMeta), &remote.ObjectMeta),
		Spec: vkv1alpha1.ShadowPodSpec{
			Pod: RemotePodSpec(local.Spec.DeepCopy(), remote.Spec.Pod.DeepCopy(), saSecretRetriever, kubernetesServiceIPRetriever),
		},
	}
}

// RemotePodSpec forges the specs of the reflected pod specs, given the local ones.
// It expects the local and remote objects to be deepcopies, as they are mutated.
func RemotePodSpec(local, remote *corev1.PodSpec, saSecretRetriever SASecretRetriever,
	kubernetesServiceIPRetriever KubernetesServiceIPGetter) corev1.PodSpec {
	// The ServiceAccountName field in the pod specifications is optional, and empty means default.
	if local.ServiceAccountName == "" {
		local.ServiceAccountName = "default"
	}

	remote.Containers = RemoteContainers(local.Containers, local.ServiceAccountName)
	remote.InitContainers = RemoteContainers(local.InitContainers, local.ServiceAccountName)

	remote.HostAliases = RemoteHostAliases(local.HostAliases, kubernetesServiceIPRetriever)
	remote.Tolerations = RemoteTolerations(local.Tolerations)
	remote.Volumes = RemoteVolumes(local.Volumes, func() string { return saSecretRetriever(local.ServiceAccountName) })

	remote.ActiveDeadlineSeconds = local.ActiveDeadlineSeconds
	remote.DNSConfig = local.DNSConfig
	remote.DNSPolicy = local.DNSPolicy
	remote.EnableServiceLinks = local.EnableServiceLinks
	remote.Hostname = local.Hostname
	remote.ImagePullSecrets = local.ImagePullSecrets
	remote.ReadinessGates = local.ReadinessGates
	remote.RestartPolicy = local.RestartPolicy
	remote.SecurityContext = local.SecurityContext
	remote.SetHostnameAsFQDN = local.SetHostnameAsFQDN
	remote.ShareProcessNamespace = local.ShareProcessNamespace
	remote.Subdomain = local.Subdomain
	remote.TerminationGracePeriodSeconds = local.TerminationGracePeriodSeconds
	remote.TopologySpreadConstraints = local.TopologySpreadConstraints

	// The information about the service account name is not reflected, since the volume is already
	// present, and the remote creation would fail as the corresponding service account is not present.
	remote.AutomountServiceAccountToken = pointer.Bool(false)

	// This fields are currently forced to false, to prevent invasive settings on the remote cluster (which might not work).
	remote.HostIPC = false
	remote.HostNetwork = false
	remote.HostPID = false

	return *remote
}

// RemoteContainers forges the containers for a reflected pod, appropriately adding the environment variables
// to enable the offloaded containers to contact back the local API server, instead of the remote one.
func RemoteContainers(containers []corev1.Container, saName string) []corev1.Container {
	for i := range containers {
		containers[i].Env = RemoteContainerEnvVariables(containers[i].Env, saName)
	}

	return containers
}

// RemoteContainerEnvVariables forges the environment variables to enable offloaded containers to
// contact back the local API server, instead of the remote one. In addition, it also hardcodes the
// service account name in case it was retrieved from the pod spec, as it is not reflected remotely.
func RemoteContainerEnvVariables(envs []corev1.EnvVar, saName string) []corev1.EnvVar {
	for i := range envs {
		if envs[i].ValueFrom != nil && envs[i].ValueFrom.FieldRef != nil &&
			envs[i].ValueFrom.FieldRef.FieldPath == "spec.serviceAccountName" {
			// Hardcode the correct service account name value, as not propagated remotely.
			envs[i].Value = saName
			envs[i].ValueFrom = nil
		}
	}

	hostport := "tcp://" + net.JoinHostPort(kubernetesAPIService, KubernetesServicePort)
	return append(envs,
		// We replace the correct IP address with the kubernetes.default hostname (which is associated with the remapped
		// IP through an appropriate host alias), since directly using the remapped IP address would lead to TLS errors
		// as it is not included in the certificate.
		corev1.EnvVar{Name: "KUBERNETES_SERVICE_HOST", Value: kubernetesAPIService},
		corev1.EnvVar{Name: "KUBERNETES_SERVICE_PORT", Value: KubernetesServicePort},
		corev1.EnvVar{Name: "KUBERNETES_PORT", Value: hostport},
		corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP", KubernetesServicePort), Value: hostport},
		corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP_PROTO", KubernetesServicePort), Value: "tcp"},
		corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP_ADDR", KubernetesServicePort), Value: kubernetesAPIService},
		corev1.EnvVar{Name: fmt.Sprintf("KUBERNETES_PORT_%s_TCP_PORT", KubernetesServicePort), Value: KubernetesServicePort},
	)
}

// RemoteHostAliases forges the host aliases to override the IP address associated with the kubernetes.default service
// to enable offloaded containers to contact back the local API server, instead of the remote one.
func RemoteHostAliases(aliases []corev1.HostAlias, kubernetesServiceIPRetriever KubernetesServiceIPGetter) []corev1.HostAlias {
	return append(aliases, corev1.HostAlias{
		IP: kubernetesServiceIPRetriever(), Hostnames: []string{kubernetesAPIService, kubernetesAPIService + ".svc"}})
}

// RemoteTolerations forges the tolerations for a reflected pod.
func RemoteTolerations(inputTolerations []corev1.Toleration) []corev1.Toleration {
	tolerations := make([]corev1.Toleration, 0)

	for _, toleration := range inputTolerations {
		// Copy all tolerations except the one for the virtual node.
		// This prevents by default the possibility of "recursive" scheduling on virtual nodes on the target cluster.
		if toleration.Key != liqoconst.VirtualNodeTolerationKey {
			tolerations = append(tolerations, toleration)
		}
	}

	return tolerations
}

// RemoteVolumes forges the volumes for a reflected pod, appropriately modifying the one related to the service account.
func RemoteVolumes(volumes []corev1.Volume, saSecretRetriever func() string) []corev1.Volume {
	for i := range volumes {
		// Modify the projected volume which refers to the service account (if any),
		// to make it target the underlying secret/configmap reflected to the remote cluster.
		if volumes[i].Projected != nil && strings.HasPrefix(volumes[i].Name, ServiceAccountVolumeName) {
			for j := range volumes[i].Projected.Sources {
				source := &volumes[i].Projected.Sources[j]
				if source.ConfigMap != nil {
					// Replace the certification authority configmap with the remapped name.
					source.ConfigMap.Name = RemoteConfigMapName(source.ConfigMap.Name)
				} else if source.ServiceAccountToken != nil {
					// Replace the ServiceAccountToken entry with the corresponding secret one.
					source.ServiceAccountToken = nil
					source.Secret = &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: saSecretRetriever()},
						Items:                []corev1.KeyToPath{{Key: corev1.ServiceAccountTokenKey, Path: corev1.ServiceAccountTokenKey}},
					}
				}
			}
		}
	}

	return volumes
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
