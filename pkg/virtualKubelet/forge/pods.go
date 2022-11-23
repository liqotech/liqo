// Copyright 2019-2023 The Liqo Authors
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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"k8s.io/utils/pointer"

	vkv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/maps"
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

// APIServerSupportType is the enum type representing which type of API Server support is enabled,
// i.e., to allow offloaded pods to contact the local API server.
type APIServerSupportType string

const (
	// APIServerSupportDisabled -> API Server support is disabled.
	APIServerSupportDisabled APIServerSupportType = "Disabled"
	// APIServerSupportLegacy -> API Server support is enabled, using the legacy secrets associated with service accounts.
	APIServerSupportLegacy APIServerSupportType = "Legacy"
	// APIServerSupportTokenAPI -> API Server support is enabled, leveraging the newer TokenRequest API to retrieve the tokens.
	APIServerSupportTokenAPI APIServerSupportType = "TokenAPI"
)

// PodIPTranslator defines the function to translate between remote and local IP addresses.
type PodIPTranslator func(string) string

// RemotePodSpecMutator defines the function type to mutate the remote pod specifications and implement additional capabilities.
type RemotePodSpecMutator func(remote *corev1.PodSpec)

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
func RemoteShadowPod(local *corev1.Pod, remote *vkv1alpha1.ShadowPod,
	targetNamespace string, mutators ...RemotePodSpecMutator) *vkv1alpha1.ShadowPod {
	var creation bool
	if remote == nil {
		// The remote is nil if not already created.
		creation = true
		remote = &vkv1alpha1.ShadowPod{ObjectMeta: metav1.ObjectMeta{Name: local.GetName(), Namespace: targetNamespace}}
	}

	// Remove the label which identifies offloaded pods, as meaningful only locally.
	localMetaFiltered := local.ObjectMeta.DeepCopy()
	delete(localMetaFiltered.GetLabels(), liqoconst.LocalPodLabelKey)

	// Initialize the appropriate anti-affinity mutator if the corresponding annotation is present.
	switch local.Annotations[liqoconst.PodAntiAffinityPresetKey] {
	case liqoconst.PodAntiAffinityPresetValuePropagate:
		mutators = append(mutators, AntiAffinityPropagateMutator(local.Spec.Affinity.DeepCopy()))
	case liqoconst.PodAntiAffinityPresetValueSoft:
		mutators = append(mutators,
			AntiAffinitySoftMutator(FilterAntiAffinityLabels(localMetaFiltered.GetLabels(), local.Annotations[liqoconst.PodAntiAffinityLabelsKey])))
	case liqoconst.PodAntiAffinityPresetValueHard:
		mutators = append(mutators,
			AntiAffinityHardMutator(FilterAntiAffinityLabels(localMetaFiltered.GetLabels(), local.Annotations[liqoconst.PodAntiAffinityLabelsKey])))
	}

	return &vkv1alpha1.ShadowPod{
		ObjectMeta: RemoteObjectMeta(localMetaFiltered, &remote.ObjectMeta),
		Spec: vkv1alpha1.ShadowPodSpec{
			Pod: RemotePodSpec(creation, local.Spec.DeepCopy(), remote.Spec.Pod.DeepCopy(), mutators...),
		},
	}
}

// RemotePodSpec forges the specs of the reflected pod specs, given the local ones.
// It expects the local and remote objects to be deepcopies, as they are mutated.
func RemotePodSpec(creation bool, local, remote *corev1.PodSpec, mutators ...RemotePodSpecMutator) corev1.PodSpec {
	// Do not mutate the pod specifications after it has been created, since it is likely the modification
	// would be rejected by the API server, as only a very limited set of fields can be mutated.
	// Additionally, such modification would not be currently propagated by the remote ShadowPod controller.
	if !creation {
		return *remote
	}

	remote.Containers = local.Containers
	remote.InitContainers = local.InitContainers

	remote.Tolerations = RemoteTolerations(local.Tolerations)
	remote.Volumes = local.Volumes

	remote.ActiveDeadlineSeconds = local.ActiveDeadlineSeconds
	remote.DNSConfig = local.DNSConfig
	remote.DNSPolicy = local.DNSPolicy
	remote.EnableServiceLinks = local.EnableServiceLinks
	remote.HostAliases = local.HostAliases
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

	// Perform the additional mutations to implement advanced functionalities.
	for _, mutator := range mutators {
		mutator(remote)
	}

	return *remote
}

// APIServerSupportMutator is a mutator which implements the support to enable offloaded pods to interact back with the local Kubernetes API server.
func APIServerSupportMutator(apiServerSupport APIServerSupportType, saName string, saSecretRetriever SASecretRetriever,
	kubernetesServiceIPRetriever KubernetesServiceIPGetter) RemotePodSpecMutator {
	return func(remote *corev1.PodSpec) {
		// The mutation of the service account related volume needs to be performed regardless of whether this feature is enabled.
		remote.Volumes = RemoteVolumes(remote.Volumes, apiServerSupport, func() string { return saSecretRetriever(saName) })

		// No additional operations need to be performed if the API server support is disabled.
		if apiServerSupport == APIServerSupportDisabled {
			return
		}

		// Mutate the environment variables of the containers concerning the target API server hostname, and the service account name.
		remote.Containers = RemoteContainersAPIServerSupport(remote.Containers, saName)
		remote.InitContainers = RemoteContainersAPIServerSupport(remote.InitContainers, saName)

		// Add a custom host alias to reach "kubernetes.default" through the remapped IP address.
		remote.HostAliases = RemoteHostAliasesAPIServerSupport(remote.HostAliases, kubernetesServiceIPRetriever)
	}
}

// AntiAffinityPropagateMutator is a mutator which implements the support to propagate a given anti-affinity constraint.
func AntiAffinityPropagateMutator(affinity *corev1.Affinity) RemotePodSpecMutator {
	return func(remote *corev1.PodSpec) {
		if affinity != nil && affinity.PodAntiAffinity != nil {
			remote.Affinity = &corev1.Affinity{PodAntiAffinity: affinity.PodAntiAffinity}
		}
	}
}

// AntiAffinitySoftMutator is a mutator which implements the support to enable soft anti-affinity between pods sharing the same labels.
func AntiAffinitySoftMutator(labels map[string]string) RemotePodSpecMutator {
	return func(remote *corev1.PodSpec) {
		remote.Affinity = &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
				Weight: 1,
				PodAffinityTerm: corev1.PodAffinityTerm{
					TopologyKey:   corev1.LabelHostname,
					LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
				},
			}},
		}}
	}
}

// AntiAffinityHardMutator is a mutator which implements the support to enable hard anti-affinity between pods sharing the same labels.
func AntiAffinityHardMutator(labels map[string]string) RemotePodSpecMutator {
	return func(remote *corev1.PodSpec) {
		remote.Affinity = &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				TopologyKey:   corev1.LabelHostname,
				LabelSelector: &metav1.LabelSelector{MatchLabels: labels},
			}},
		}}
	}
}

// FilterAntiAffinityLabels filters the label keys which are used to implement the anti-affinity constraints, based on the specified whitelist.
func FilterAntiAffinityLabels(labels map[string]string, whitelist string) map[string]string {
	if whitelist != "" {
		return maps.Filter(labels, maps.FilterWhitelist(strings.Split(whitelist, ",")...))
	}

	return maps.Filter(labels, maps.FilterBlacklist(appsv1.ControllerRevisionHashLabelKey,
		appsv1.DefaultDeploymentUniqueLabelKey, appsv1.StatefulSetPodNameLabel))
}

// RemoteContainersAPIServerSupport forges the containers for a reflected pod, appropriately adding the environment variables
// to enable the offloaded containers to contact back the local API server, instead of the remote one.
func RemoteContainersAPIServerSupport(containers []corev1.Container, saName string) []corev1.Container {
	for i := range containers {
		containers[i].Env = RemoteContainerEnvVariablesAPIServerSupport(containers[i].Env, saName)
	}

	return containers
}

// RemoteContainerEnvVariablesAPIServerSupport forges the environment variables to enable offloaded containers to
// contact back the local API server, instead of the remote one. In addition, it also hardcodes the
// service account name in case it was retrieved from the pod spec, as it is not reflected remotely.
func RemoteContainerEnvVariablesAPIServerSupport(envs []corev1.EnvVar, saName string) []corev1.EnvVar {
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

// RemoteHostAliasesAPIServerSupport forges the host aliases to override the IP address associated with the kubernetes.default
// service to enable offloaded containers to contact back the local API server, instead of the remote one.
func RemoteHostAliasesAPIServerSupport(aliases []corev1.HostAlias, retriever KubernetesServiceIPGetter) []corev1.HostAlias {
	return append(aliases, corev1.HostAlias{
		IP: retriever(), Hostnames: []string{kubernetesAPIService, kubernetesAPIService + ".svc"}})
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
func RemoteVolumes(volumes []corev1.Volume, apiServerSupport APIServerSupportType, saSecretRetriever func() string) []corev1.Volume {
	for i := range volumes {
		// Modify the projected volume which refers to the service account (if any),
		// to make it target the underlying secret/configmap reflected to the remote cluster.
		if volumes[i].Projected != nil {
			var offset int
			for j := range volumes[i].Projected.Sources {
				j -= offset // Account for the entry that might have been previously deleted.
				source := &volumes[i].Projected.Sources[j]
				if source.ConfigMap != nil {
					// Replace the certification authority configmap with the remapped name.
					source.ConfigMap.Name = RemoteConfigMapName(source.ConfigMap.Name)
				} else if source.ServiceAccountToken != nil {
					if apiServerSupport == APIServerSupportDisabled ||
						// Tokens different from the kube-api-access one are not supported in legacy mode.
						(apiServerSupport == APIServerSupportLegacy && !strings.HasPrefix(volumes[i].Name, ServiceAccountVolumeName)) {
						// Remove the entry referring to the service account.
						volumes[i].Projected.Sources = append(volumes[i].Projected.Sources[:j], volumes[i].Projected.Sources[j+1:]...)
						offset++
						continue
					}

					// Replace the ServiceAccountToken entry with the corresponding secret one, only in case it is enabled.
					secretKey := corev1.ServiceAccountTokenKey
					path := source.ServiceAccountToken.Path
					if apiServerSupport == APIServerSupportTokenAPI {
						secretKey = ServiceAccountTokenKey(volumes[i].Name, path)
					}

					source.ServiceAccountToken = nil
					source.Secret = &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: saSecretRetriever()},
						Items:                []corev1.KeyToPath{{Key: secretKey, Path: path}},
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
