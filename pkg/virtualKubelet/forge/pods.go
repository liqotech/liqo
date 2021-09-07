package forge

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	liqoconst "github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/virtualKubelet"
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/reflectors"
)

const affinitySelector = liqoconst.TypeNode

func (f *apiForger) podForeignToHome(foreignObj, homeObj runtime.Object, reflectionType string) (*corev1.Pod, error) {
	var isNewObject bool

	if homeObj == nil {
		isNewObject = true

		homeObj = &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       corev1.PodSpec{},
		}
	}

	foreignPod := foreignObj.(*corev1.Pod)
	homePod := homeObj.(*corev1.Pod)

	foreignNamespace, err := f.nattingTable.DeNatNamespace(foreignPod.Namespace)
	if err != nil {
		return nil, err
	}

	f.forgeHomeMeta(&foreignPod.ObjectMeta, &homePod.ObjectMeta, foreignNamespace, reflectionType)
	delete(homePod.Labels, virtualKubelet.ReflectedpodKey)

	if isNewObject {
		homePod.Spec = f.forgePodSpec(foreignPod.Spec)
	}

	return homePod, nil
}

func (f *apiForger) podStatusForeignToHome(foreignObj, homeObj runtime.Object) *corev1.Pod {
	homePod := homeObj.(*corev1.Pod)
	foreignPod := foreignObj.(*corev1.Pod)

	homePod.Status = foreignPod.Status
	if homePod.Status.PodIP != "" {
		response, err := f.ipamClient.GetHomePodIP(context.Background(),
			&liqonetIpam.GetHomePodIPRequest{
				ClusterID: strings.TrimPrefix(f.virtualNodeName.Value().ToString(), virtualKubelet.VirtualNodePrefix),
				Ip:        foreignPod.Status.PodIP,
			})
		if err != nil {
			klog.Error(err)
		}
		homePod.Status.PodIP = response.GetHomeIP()
		homePod.Status.PodIPs[0].IP = response.GetHomeIP()
	}

	if foreignPod.DeletionTimestamp != nil {
		homePod.DeletionTimestamp = nil
		foreignKey := fmt.Sprintf("%s/%s", foreignPod.Namespace, foreignPod.Name)
		reflectors.Blacklist[apimgmt.Pods][foreignKey] = struct{}{}
		klog.V(3).Infof("pod %s blacklisted because marked for deletion", foreignKey)
	}

	return homePod
}

// Set pod's container statutes to terminated so that the pod can be deleted.
func (f *apiForger) setPodToBeDeleted(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Phase = corev1.PodUnknown
	for i := range pod.Status.ContainerStatuses {
		pod.Status.ContainerStatuses[i].State = corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{},
		}
	}

	return pod
}

func (f *apiForger) podHomeToForeign(homeObj, foreignObj runtime.Object, reflectionType string) (*corev1.Pod, error) {
	var isNewObject bool
	var homePod, foreignPod *corev1.Pod

	if foreignObj == nil {
		isNewObject = true

		foreignPod = &corev1.Pod{
			TypeMeta:   metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{},
			Spec:       corev1.PodSpec{},
		}
	} else {
		foreignPod = foreignObj.(*corev1.Pod)
	}

	homePod = homeObj.(*corev1.Pod)

	foreignNamespace, err := f.nattingTable.NatNamespace(homePod.Namespace)
	if err != nil {
		return nil, err
	}

	f.forgeForeignMeta(&homePod.ObjectMeta, &foreignPod.ObjectMeta, foreignNamespace, reflectionType)

	if isNewObject {
		foreignPod.Spec = f.forgePodSpec(homePod.Spec)
		foreignPod.Spec.Affinity = forgeAffinity()
	}

	return foreignPod, nil
}

func (f *apiForger) forgePodSpec(inputPodSpec corev1.PodSpec) corev1.PodSpec {
	outputPodSpec := corev1.PodSpec{}

	outputPodSpec.TerminationGracePeriodSeconds = inputPodSpec.TerminationGracePeriodSeconds
	outputPodSpec.Volumes = forgeVolumes(inputPodSpec.Volumes)
	outputPodSpec.InitContainers = forgeContainers(inputPodSpec.InitContainers, outputPodSpec.Volumes)
	outputPodSpec.Containers = forgeContainers(inputPodSpec.Containers, outputPodSpec.Volumes)
	outputPodSpec.Tolerations = forgeTolerations(inputPodSpec.Tolerations)

	return outputPodSpec
}

func forgeTolerations(inputTolerations []corev1.Toleration) []corev1.Toleration {
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
		if v.ConfigMap != nil || v.EmptyDir != nil || v.DownwardAPI != nil {
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

func forgeAffinity() *corev1.Affinity {
	return &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      liqoconst.TypeLabel,
								Operator: corev1.NodeSelectorOpNotIn,
								Values:   []string{affinitySelector},
							},
						},
					},
				},
			},
		},
	}
}
