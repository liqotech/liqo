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

package virtualnode

import (
	"slices"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

func (w *vnwh) initVirtualNodeDeployment(vn *offloadingv1beta1.VirtualNode, opts *offloadingv1beta1.VkOptionsTemplate) {
	if vn.Spec.Template == nil {
		vn.Spec.Template = &offloadingv1beta1.DeploymentTemplate{}
	}
	vkdep := vkforge.VirtualKubeletDeployment(w.clusterID, w.localPodCIDR, w.liqoNamespace, vn, opts)
	vn.Spec.Template.Spec = *vkdep.Spec.DeepCopy()
	vn.Spec.Template.ObjectMeta = *vkdep.ObjectMeta.DeepCopy()
}

func mutateSpec(vn *offloadingv1beta1.VirtualNode, opts *offloadingv1beta1.VkOptionsTemplate) {
	if vn.Spec.CreateNode == nil {
		vn.Spec.CreateNode = &opts.Spec.CreateNode
	}
	if vn.Spec.DisableNetworkCheck == nil {
		vn.Spec.DisableNetworkCheck = &opts.Spec.DisableNetworkCheck
	}

	mutateOffloadingPatch(vn, opts)
}

func mutateOffloadingPatch(vn *offloadingv1beta1.VirtualNode, opts *offloadingv1beta1.VkOptionsTemplate) {
	if vn.Spec.OffloadingPatch == nil {
		vn.Spec.OffloadingPatch = &offloadingv1beta1.OffloadingPatch{}
	}

	// Add labels not reflected from opts if not already present in the VN OffloadingPatch.
	for i := range opts.Spec.LabelsNotReflected {
		if !slices.Contains(vn.Spec.OffloadingPatch.LabelsNotReflected, opts.Spec.LabelsNotReflected[i]) {
			vn.Spec.OffloadingPatch.LabelsNotReflected = append(vn.Spec.OffloadingPatch.LabelsNotReflected, opts.Spec.LabelsNotReflected[i])
		}
	}

	// Add annotations not reflected from opts if not already present in the VN OffloadingPatch.
	for i := range opts.Spec.AnnotationsNotReflected {
		if !slices.Contains(vn.Spec.OffloadingPatch.AnnotationsNotReflected, opts.Spec.AnnotationsNotReflected[i]) {
			vn.Spec.OffloadingPatch.AnnotationsNotReflected = append(vn.Spec.OffloadingPatch.AnnotationsNotReflected, opts.Spec.AnnotationsNotReflected[i])
		}
	}
}

func overrideVKOptionsFromExistingVirtualNode(opts *offloadingv1beta1.VkOptionsTemplate, vn *offloadingv1beta1.VirtualNode) {
	if vn.Spec.Template == nil {
		return
	}

	overrideVKOptionsMetadata(opts, &vn.Spec.Template.ObjectMeta)
	overrideVKOptionsSpec(opts, &vn.Spec.Template.Spec)
}

func overrideVKOptionsSpec(opts *offloadingv1beta1.VkOptionsTemplate, depSpec *appsv1.DeploymentSpec) {
	if len(depSpec.Template.Spec.Containers) == 0 {
		return
	}

	container := depSpec.Template.Spec.Containers[0]
	if container.Image != "" {
		opts.Spec.ContainerImage = container.Image
	}
	overrideVKOptionsResources(opts, &container.Resources)
	overrideVKOptionsArgs(opts, container.Args)
}

func overrideVKOptionsMetadata(opts *offloadingv1beta1.VkOptionsTemplate, depMeta *metav1.ObjectMeta) {
	if depMeta == nil {
		return
	}
	if depMeta.Labels != nil {
		if opts.Spec.ExtraLabels == nil {
			opts.Spec.ExtraLabels = make(map[string]string)
		}
		for k, v := range depMeta.Labels {
			opts.Spec.ExtraLabels[k] = v
		}
	}
	if depMeta.Annotations != nil {
		if opts.Spec.ExtraAnnotations == nil {
			opts.Spec.ExtraAnnotations = make(map[string]string)
		}
		for k, v := range depMeta.Annotations {
			opts.Spec.ExtraAnnotations[k] = v
		}
	}
}

func overrideVKOptionsResources(opts *offloadingv1beta1.VkOptionsTemplate, res *corev1.ResourceRequirements) {
	if res == nil {
		return
	}

	if res.Limits != nil {
		for k, v := range res.Limits {
			if opts.Spec.Resources.Limits == nil {
				opts.Spec.Resources.Limits = make(corev1.ResourceList)
			}
			opts.Spec.Resources.Limits[k] = v.DeepCopy()
		}
	}
	if res.Requests != nil {
		for k, v := range res.Requests {
			if opts.Spec.Resources.Requests == nil {
				opts.Spec.Resources.Requests = make(corev1.ResourceList)
			}
			opts.Spec.Resources.Requests[k] = v.DeepCopy()
		}
	}
}

func overrideVKOptionsArgs(opts *offloadingv1beta1.VkOptionsTemplate, args []string) {
	for i := range args {
		k, v := vkforge.DestringifyArgument(args[i])
		switch k {
		case string(vkforge.NodeExtraAnnotations):
			for _, annotation := range strings.Split(v, ",") {
				vk := strings.Split(annotation, "=")
				if opts.Spec.NodeExtraAnnotations == nil {
					opts.Spec.NodeExtraAnnotations = make(map[string]string)
				}
				opts.Spec.NodeExtraAnnotations[vk[0]] = vk[1]
			}
		case string(vkforge.NodeExtraLabels):
			if opts.Spec.NodeExtraLabels == nil {
				opts.Spec.NodeExtraLabels = make(map[string]string)
			}
			for _, label := range strings.Split(v, ",") {
				vk := strings.Split(label, "=")
				opts.Spec.NodeExtraLabels[vk[0]] = vk[1]
			}
		case string(vkforge.MetricsEnabled):
			opts.Spec.MetricsEnabled = true
		case string(vkforge.MetricsAddress):
			opts.Spec.MetricsAddress = v
		}
	}

	for i := range opts.Spec.ExtraArgs {
		extraArgKey, _ := vkforge.DestringifyArgument(opts.Spec.ExtraArgs[i])
		for j := range args {
			argKey, _ := vkforge.DestringifyArgument(args[j])
			if extraArgKey == argKey {
				opts.Spec.ExtraArgs[i] = args[j]
				break
			}
		}
	}
}

func mutateSpecInTemplate(vn *offloadingv1beta1.VirtualNode, vkOpts *offloadingv1beta1.VkOptionsTemplate) {
	mutateSecretArg(vn)
	mutateNodeCreate(vn)
	mutateNodeCheckNetwork(vn)
	mutateReplicas(vn, vkOpts)
}

func mutateReplicas(vn *offloadingv1beta1.VirtualNode, vkOpts *offloadingv1beta1.VkOptionsTemplate) {
	if vkOpts.Spec.Replicas != nil {
		vn.Spec.Template.Spec.Replicas = vkOpts.Spec.Replicas
	}
}

// mutateSecretArg mutate the foreigncluster kubeconfig secret name in the virtual kubelet deployment.
func mutateSecretArg(vn *offloadingv1beta1.VirtualNode) {
	ksref := vn.Spec.KubeconfigSecretRef
	if ksref == nil {
		return
	}
	argSecret := vkforge.StringifyArgument(string(vkforge.ForeignClusterKubeconfigSecretName), ksref.Name)
	container := &vn.Spec.Template.Spec.Template.Spec.Containers[0]

	for i, arg := range container.Args {
		if strings.HasPrefix(arg, string(vkforge.ForeignClusterKubeconfigSecretName)) {
			if arg == argSecret {
				return
			}
			container.Args[i] = argSecret
			return
		}
	}

	container.Args = append(container.Args, argSecret)
}

// mutateNodeCreate mutate the creation of the remote cluster node.
func mutateNodeCreate(vn *offloadingv1beta1.VirtualNode) {
	argCreateNode := vkforge.StringifyArgument(string(vkforge.CreateNode), strconv.FormatBool(*vn.Spec.CreateNode))
	container := &vn.Spec.Template.Spec.Template.Spec.Containers[0]
	for i, arg := range container.Args {
		if strings.HasPrefix(arg, string(vkforge.CreateNode)) {
			if arg == argCreateNode {
				return
			}
			container.Args[i] = argCreateNode
			return
		}
	}

	container.Args = append(container.Args, argCreateNode)
}

// mutateNodeCheckNetwork flag mutate the check network flag.
func mutateNodeCheckNetwork(vn *offloadingv1beta1.VirtualNode) {
	argCheckNetwork := vkforge.StringifyArgument(string(vkforge.NodeCheckNetwork), strconv.FormatBool(!*vn.Spec.DisableNetworkCheck))
	container := &vn.Spec.Template.Spec.Template.Spec.Containers[0]
	for i, arg := range container.Args {
		if strings.HasPrefix(arg, string(vkforge.NodeCheckNetwork)) {
			if arg == argCheckNetwork {
				return
			}
			container.Args[i] = argCheckNetwork
			return
		}
	}

	container.Args = append(container.Args, argCheckNetwork)
}
