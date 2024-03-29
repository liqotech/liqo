// Copyright 2019-2024 The Liqo Authors
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
	"fmt"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	vkforge "github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

func (w *vnwh) initVirtualNode(virtualNode *virtualkubeletv1alpha1.VirtualNode) {
	w.mutateVirtualNodeSpecTemplate(virtualNode)
}

func (w *vnwh) mutateVirtualNodeSpecTemplate(virtualNode *virtualkubeletv1alpha1.VirtualNode) {
	if virtualNode.Spec.Template == nil {
		virtualNode.Spec.Template = &virtualkubeletv1alpha1.DeploymentTemplate{}
	}
	vkdep := vkforge.VirtualKubeletDeployment(w.clusterIdentity, virtualNode, w.virtualKubeletOptions)
	virtualNode.Spec.Template.Spec = *vkdep.Spec.DeepCopy()
	virtualNode.Spec.Template.ObjectMeta = *vkdep.ObjectMeta.DeepCopy()
}

func mutateVKOptionsResources(opts *vkforge.VirtualKubeletOpts, res *corev1.ResourceRequirements) {
	if res == nil {
		return
	}
	if res.Limits != nil {
		if res.Limits.Cpu() != nil {
			opts.LimitsCPU = res.Limits.Cpu().DeepCopy()
		}
		if res.Limits.Memory() != nil {
			opts.LimitsRAM = res.Limits.Memory().DeepCopy()
		}
	}
	if res.Requests != nil {
		if res.Requests.Cpu() != nil {
			opts.RequestsCPU = res.Requests.Cpu().DeepCopy()
		}
		if res.Requests.Memory() != nil {
			opts.RequestsRAM = res.Requests.Memory().DeepCopy()
		}
	}
}

func mutateVKOptionsMetadata(opts *vkforge.VirtualKubeletOpts, meta *metav1.ObjectMeta) {
	if meta == nil {
		return
	}
	if meta.Labels != nil {
		if opts.ExtraLabels == nil {
			opts.ExtraLabels = make(map[string]string)
		}
		for k, v := range meta.Labels {
			opts.ExtraLabels[k] = v
		}
	}
	if meta.Annotations != nil {
		if opts.ExtraAnnotations == nil {
			opts.ExtraAnnotations = make(map[string]string)
		}
		for k, v := range meta.Annotations {
			opts.ExtraAnnotations[k] = v
		}
	}
}

func mutateVKOptionsFlags(opts *vkforge.VirtualKubeletOpts, container *corev1.Container) {
	for _, arg := range container.Args {
		if found := strings.HasPrefix(arg, string(vkforge.IpamEndpoint)); found {
			value := strings.TrimPrefix(arg, string(vkforge.IpamEndpoint))
			opts.IpamEndpoint = strings.TrimLeft(value, " ")
		} else if found := strings.HasPrefix(arg, string(vkforge.NodeExtraAnnotations)); found {
			value := strings.TrimPrefix(arg, string(vkforge.NodeExtraAnnotations))
			annotations := strings.TrimLeft(value, " ")
			for _, annotation := range strings.Split(annotations, ",") {
				vk := strings.Split(annotation, "=")
				opts.NodeExtraAnnotations.StringMap[vk[0]] = vk[1]
			}
		} else if found := strings.HasPrefix(arg, string(vkforge.NodeExtraLabels)); found {
			value := strings.TrimPrefix(arg, string(vkforge.NodeExtraLabels))
			labels := strings.TrimLeft(value, " ")
			for _, label := range strings.Split(labels, ",") {
				vk := strings.Split(label, "=")
				opts.NodeExtraLabels.StringMap[vk[0]] = vk[1]
			}
		} else if found := strings.HasPrefix(arg, string(vkforge.MetricsEnabled)); found {
			opts.MetricsEnabled = found
		} else if found := strings.HasPrefix(arg, string(vkforge.MetricsAddress)); found {
			value := strings.TrimPrefix(arg, string(vkforge.MetricsAddress))
			opts.MetricsAddress = strings.TrimLeft(value, " ")
		}
	}
}

func mutateVKOptions(opts *vkforge.VirtualKubeletOpts, vn *virtualkubeletv1alpha1.VirtualNode) {
	if vn.Spec.Template == nil {
		return
	}
	mutateVKOptionsMetadata(opts, &vn.Spec.Template.ObjectMeta)
	if len(vn.Spec.Template.Spec.Template.Spec.Containers) == 1 {
		container := vn.Spec.Template.Spec.Template.Spec.Containers[0]
		opts.ContainerImage = container.Image
		mutateVKOptionsResources(opts, &container.Resources)
		mutateVKOptionsFlags(opts, &container)
	}
}

func mutateSpecInTemplate(vn *virtualkubeletv1alpha1.VirtualNode) {
	mutateSecretArg(vn)
	mutateNodeCreate(vn)
}

// mutateSecretArg mutate the foreigncluster kubeconfig secret name in the virtual kubelet deployment.
func mutateSecretArg(vn *virtualkubeletv1alpha1.VirtualNode) {
	ksref := vn.Spec.KubeconfigSecretRef
	if ksref == nil {
		return
	}
	argSecret := fmt.Sprintf("%s=%s", vkforge.ForeignClusterKubeconfigSecretName, ksref.Name)
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
func mutateNodeCreate(vn *virtualkubeletv1alpha1.VirtualNode) {
	argCreateNode := fmt.Sprintf("%s=%s", vkforge.CreateNode, strconv.FormatBool(*vn.Spec.CreateNode))
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
