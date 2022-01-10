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

package mutate

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
)

// getVirtualNodeToleration returns a new Toleration for the Liqo's virtual-nodes.
func getVirtualNodeToleration() corev1.Toleration {
	return corev1.Toleration{
		Key:      liqoconst.VirtualNodeTolerationKey,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoExecute,
	}
}

// createTolerationFromNamespaceOffloading creates a new virtualNodeToleration in case of LocalAndRemotePodOffloadingStrategyType
// or RemotePodOffloadingStrategyType. In case of PodOffloadingStrategyType not recognized, returns an error.
func createTolerationFromNamespaceOffloading(strategy offv1alpha1.PodOffloadingStrategyType) (corev1.Toleration, error) {
	var toleration corev1.Toleration
	switch {
	case strategy == offv1alpha1.LocalAndRemotePodOffloadingStrategyType, strategy == offv1alpha1.RemotePodOffloadingStrategyType:
		// The virtual-node toleration must be added.
		toleration = getVirtualNodeToleration()
	default:
		err := fmt.Errorf("PodOffloadingStrategyType '%s' not recognized", strategy)
		klog.Error(err)
		return corev1.Toleration{}, err
	}
	return toleration, nil
}

// createNodeSelectorFromNamespaceOffloading creates the right NodeSelector according to the PodOffloadingStrategy chosen.
func createNodeSelectorFromNamespaceOffloading(nsoff *offv1alpha1.NamespaceOffloading) (corev1.NodeSelector, error) {
	nodeSelector := nsoff.Spec.ClusterSelector
	switch {
	case nsoff.Spec.PodOffloadingStrategy == offv1alpha1.RemotePodOffloadingStrategyType:
		// To ensure that the pod is not scheduled on local nodes is necessary to add to every NodeSelectorTerm a
		// new NodeSelectorRequirement. This NodeSelectorRequirement requires explicitly the label
		// "liqo.io/type=virtual-node" to exclude local nodes from the scheduler choice.
		for i := range nodeSelector.NodeSelectorTerms {
			nodeSelector.NodeSelectorTerms[i].MatchExpressions = append(nodeSelector.NodeSelectorTerms[i].MatchExpressions,
				corev1.NodeSelectorRequirement{
					Key:      liqoconst.TypeLabel,
					Operator: corev1.NodeSelectorOpIn,
					Values:   []string{liqoconst.TypeNode},
				})
		}
	case nsoff.Spec.PodOffloadingStrategy == offv1alpha1.LocalAndRemotePodOffloadingStrategyType:
		// A new NodeSelectorTerm that allows scheduling the pod also on local nodes is added.
		newNodeSelectorTerm := corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key:      liqoconst.TypeLabel,
				Operator: corev1.NodeSelectorOpNotIn,
				Values:   []string{liqoconst.TypeNode},
			}},
		}
		nodeSelector.NodeSelectorTerms = append(nodeSelector.NodeSelectorTerms, newNodeSelectorTerm)
	default:
		err := fmt.Errorf("PodOffloadingStrategyType '%s' not recognized", nsoff.Spec.PodOffloadingStrategy)
		klog.Error(err)
		return corev1.NodeSelector{}, err
	}
	return nodeSelector, nil
}

// fillPodWithTheNewNodeSelector gets the previously computed NodeSelector imposed by the PodOffloadingStrategy and
// merges it with the Pod NodeSelector if it is already present. It simply adds it to the Pod if previously unset.
func fillPodWithTheNewNodeSelector(imposedNodeSelector *corev1.NodeSelector, pod *corev1.Pod) {
	// To preserve the Pod Affinity content, it is necessary to add the imposedNodeSelector according to what
	// is already present in the Pod Affinity.
	switch {
	case pod.Spec.Affinity == nil:
		pod.Spec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: imposedNodeSelector.DeepCopy(),
			},
		}
	case pod.Spec.Affinity.NodeAffinity == nil:
		pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: imposedNodeSelector.DeepCopy(),
		}
	case pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil ||
		len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0:
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = imposedNodeSelector.DeepCopy()
	default:
		*pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution =
			utils.MergeNodeSelector(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
				imposedNodeSelector)
	}
}

// mutatePod checks the NamespaceOffloading CR associated with the Pod's Namespace.
// The pod is modified in different ways according to the PodOffloadingStrategyType
// chosen in the CR. Two possible modifications:
// - The VirtualNodeToleration is added to the Pod Toleration if necessary.
// - The old Pod NodeSelector is substituted with a new one according to the PodOffloadingStrategyType.
func mutatePod(namespaceOffloading *offv1alpha1.NamespaceOffloading, pod *corev1.Pod) error {
	// The NamespaceOffloading CR contains information about the PodOffloadingStrategy and
	// the NodeSelector inserted by the user (ClusterSelector field).
	klog.V(5).Infof("Chosen strategy: %s", namespaceOffloading.Spec.PodOffloadingStrategy)

	// If strategy is equal to LocalPodOffloadingStrategy there is nothing to do
	if namespaceOffloading.Spec.PodOffloadingStrategy == offv1alpha1.LocalPodOffloadingStrategyType {
		return nil
	}

	// Create the right Toleration according to the PodOffloadingStrategy case.
	toleration, err := createTolerationFromNamespaceOffloading(namespaceOffloading.Spec.PodOffloadingStrategy)
	if err != nil {
		klog.Errorf("The NamespaceOffloading in namespace '%s' has unknown strategy '%s'",
			namespaceOffloading.Namespace, namespaceOffloading.Spec.PodOffloadingStrategy)
		return err
	}
	klog.V(5).Infof("Generated Toleration: %s", toleration)

	// Create the right NodeSelector according to the PodOffloadingStrategy case.
	imposedNodeSelector, err := createNodeSelectorFromNamespaceOffloading(namespaceOffloading)
	if err != nil {
		klog.Errorf("The NamespaceOffloading in namespace '%s' has unknown strategy '%s'",
			namespaceOffloading.Namespace, namespaceOffloading.Spec.PodOffloadingStrategy)
		return err
	}
	klog.V(5).Infof("ImposedNodeSelector: %s", imposedNodeSelector)

	// It is necessary to add the just created toleration.
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, toleration)

	// Enforce the new NodeSelector policy imposed by the NamespaceOffloading creator.
	fillPodWithTheNewNodeSelector(&imposedNodeSelector, pod)
	klog.V(5).Infof("Pod NodeSelector: %s", imposedNodeSelector)
	return nil
}
