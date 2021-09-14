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

package liqodeploymentctrl

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutils "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	nodeselectorutils "github.com/liqotech/liqo/pkg/utils/nodeSelector"
)

const (
	replicatorLabel = "replicator"
)

// enforceDeploymentReplicas creates a deployment replica for every combination of labels.
func (r *LiqoDeploymentReconciler) enforceDeploymentReplicas(ctx context.Context,
	ldp *offv1alpha1.LiqoDeployment, combinationsMap map[string]struct{}) bool {
	var err error
	var deployment *appsv1.Deployment
	creationNotCompleted := false

	for labelsString := range combinationsMap {
		generationLabelsString := strings.Split(labelsString, labelSeparator)
		generationLabelsString = generationLabelsString[1:]
		generationLabelsMap := map[string]string{}

		for i := range generationLabelsString {
			tmp := strings.Split(generationLabelsString[i], keyValueSeparator)
			generationLabelsMap[tmp[0]] = tmp[1]
		}

		deploymentList := &appsv1.DeploymentList{}
		if err = r.List(ctx, deploymentList, client.MatchingLabels{replicatorLabel: ldp.Name}, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(generationLabelsMap),
			Namespace:     ldp.Namespace,
		}); err != nil {
			klog.Errorf(" %s --> Unable to List deployments in the namespace %s", err, ldp.Namespace)
			creationNotCompleted = true
			continue
		}

		if deployment, err = defineNewDeployment(ldp, generationLabelsMap, r.Scheme); err != nil {
			creationNotCompleted = true
			continue
		}

		// Create the new deployment.
		if len(deploymentList.Items) == 0 {
			if err = r.Create(ctx, deployment); err != nil {
				klog.Errorf("%s -> Unable to create a deployment for the LiqoDeployment %s", err, ldp.Name)
				creationNotCompleted = true
				continue
			}
		} else { // Enforce the LiqoDeployment template on an existing deployment replica.
			original := deploymentList.Items[0].DeepCopy()
			deploymentList.Items[0].Spec = deployment.Spec
			deploymentList.Items[0].Labels = deployment.Labels
			if err = r.Patch(ctx, &deploymentList.Items[0], client.MergeFrom(original)); err != nil {
				klog.Errorf("%s -> Unable to update a deployment for the LiqoDeployment %s", err, ldp.Name)
				creationNotCompleted = true
				continue
			}
			deployment = &deploymentList.Items[0]
		}
		updateGeneratedDeploymentStatus(ldp, deployment, generationLabelsMap)
	}
	return creationNotCompleted
}

// mergeClusterFilterWithTheCombinationLabel merges the NodeSelector specified in the SelectedCluster field
// of the LiqoDeployment with MatchExpressions necessary to offload the deployment replica on a specific target.
func mergeClusterFilterWithTheCombinationLabel(ns *corev1.NodeSelector, generationLabels map[string]string) corev1.NodeSelector {
	var matchExpressions []corev1.NodeSelectorRequirement
	for key, value := range generationLabels {
		matchExpressions = append(matchExpressions, corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{value},
		})
	}

	if len(ns.NodeSelectorTerms) > 0 {
		for i := range ns.NodeSelectorTerms {
			ns.NodeSelectorTerms[i].MatchExpressions = append(ns.NodeSelectorTerms[i].MatchExpressions, matchExpressions...)
		}
	} else {
		ns = &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: matchExpressions}}}
	}

	return *ns
}

// defineNewDeployment defines the new deployment to be replicated.
func defineNewDeployment(ldp *offv1alpha1.LiqoDeployment, generationLabels map[string]string,
	s *runtime.Scheme) (*appsv1.Deployment, error) {
	imposedNodeSelector := mergeClusterFilterWithTheCombinationLabel(ldp.Spec.SelectedClusters.DeepCopy(), generationLabels)
	deploymentTemplate := ldp.Spec.Template.DeepCopy()
	// The pod nodeAffinity must be modified to schedule correctly pods.
	modifyPodNodeAffinity(imposedNodeSelector.DeepCopy(), &deploymentTemplate.Spec.Template.Spec)

	// It is necessary to add the generation labels and the LiqoDeployment representative label to:
	// - Deployment labels.
	// - Pods labels.
	if len(deploymentTemplate.Labels) == 0 {
		deploymentTemplate.Labels = map[string]string{}
	}
	deploymentTemplate.Labels[replicatorLabel] = ldp.Name

	// The LiqoDeployment representative label is also specified in the deployment selector.
	if len(deploymentTemplate.Spec.Selector.MatchLabels) == 0 {
		deploymentTemplate.Spec.Selector.MatchLabels = map[string]string{}
	}
	deploymentTemplate.Spec.Selector.MatchLabels[replicatorLabel] = ldp.Name

	if len(deploymentTemplate.Spec.Template.Labels) == 0 {
		deploymentTemplate.Spec.Template.Labels = map[string]string{}
	}
	deploymentTemplate.Spec.Template.Labels[replicatorLabel] = ldp.Name

	for k, v := range generationLabels {
		deploymentTemplate.Labels[k] = v
		deploymentTemplate.Spec.Template.Labels[k] = v
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    fmt.Sprintf("%s-", ldp.Name),
			Namespace:       ldp.Namespace,
			Labels:          deploymentTemplate.Labels,
			Annotations:     deploymentTemplate.Annotations,
			OwnerReferences: deploymentTemplate.OwnerReferences,
			Finalizers:      deploymentTemplate.Finalizers,
			ClusterName:     deploymentTemplate.ClusterName,
			ManagedFields:   deploymentTemplate.ManagedFields,
		},
		Spec: deploymentTemplate.Spec,
	}

	// The LiqoDeployment resource has the ownership on all replicated deployments.
	if err := ctrlutils.SetControllerReference(ldp, deployment, s); err != nil {
		klog.Errorf("%s -> Unable to set the controller reference for a deployment of the LiqoDeployment %s", err, ldp.Name)
		return nil, err
	}

	return deployment, nil
}

// modifyPodNodeAffinity adds the new NodeAffinity with those already owned by the pod template, if any.
func modifyPodNodeAffinity(imposedNodeSelector *corev1.NodeSelector, podSpec *corev1.PodSpec) {
	switch {
	case podSpec.Affinity == nil:
		podSpec.Affinity = &corev1.Affinity{
			NodeAffinity: &corev1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: imposedNodeSelector.DeepCopy(),
			},
		}
	case podSpec.Affinity.NodeAffinity == nil:
		podSpec.Affinity.NodeAffinity = &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: imposedNodeSelector.DeepCopy(),
		}
	case podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil ||
		len(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) == 0:
		podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = imposedNodeSelector.DeepCopy()
	default:
		*podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution =
			nodeselectorutils.GetMergedNodeSelector(podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution,
				imposedNodeSelector)
	}
}
