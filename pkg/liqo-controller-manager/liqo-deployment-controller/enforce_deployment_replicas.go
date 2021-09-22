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
)

const (
	replicatorLabel = "replicator"
)

//
func (r *LiqoDeploymentReconciler) enforceDeploymentReplicas(ctx context.Context, ldp *offv1alpha1.LiqoDeployment) bool {
	creationNotCompleted := false
	for labelsString := range r.SelectedClusters {
		generationLabelsString := strings.Split(labelsString, labelSeparator)
		generationLabelsString = generationLabelsString[1:]
		generationLabelsMap := map[string]string{}
		deploymentFinalLabels := map[string]string{}

		for i := range generationLabelsString {
			tmp := strings.Split(generationLabelsString[i], keyValueSeparator)
			generationLabelsMap[tmp[0]] = tmp[1]
			deploymentFinalLabels[tmp[0]] = tmp[1]
		}

		// The LiqoDeployment name is unique in the namespace
		deploymentFinalLabels[replicatorLabel] = ldp.Name

		// Labels already present in the deployment spec, they must be immutable.
		for k, v := range ldp.Spec.Template.Labels {
			deploymentFinalLabels[k] = v
		}

		deploymentList := &appsv1.DeploymentList{}
		if err := r.List(ctx, deploymentList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(deploymentFinalLabels),
			Namespace:     ldp.Namespace,
		}); err != nil {
			klog.Errorf(" %s --> Unable to List deployments in the namespace %s", err, ldp.Namespace)
			creationNotCompleted = true
			continue
		}

		if len(deploymentList.Items) == 0 || len(deploymentList.Items[0].Labels) != len(deploymentFinalLabels) {
			var err error
			var deployment *appsv1.Deployment
			if err, deployment = defineNewDeployment(ldp, deploymentFinalLabels, r.Scheme); err != nil {
				creationNotCompleted = true
				continue
			}

			if err = r.Create(ctx, deployment); err != nil {
				klog.Errorf("%s -> Unable to create a deployment for the LiqoDeployment %s", err, ldp.Name)
				creationNotCompleted = true
				continue
			}

			updateGeneratedDeploymentStatus(ldp, deployment, generationLabelsMap)
			continue
		}

		updateGeneratedDeploymentStatus(ldp, &deploymentList.Items[0], generationLabelsMap)
	}
	return creationNotCompleted
}

//
func defineNewDeployment(ldp *offv1alpha1.LiqoDeployment, labels map[string]string,
	s *runtime.Scheme) (error, *appsv1.Deployment) {

	// generate the required NodeSelector.
	var matchExpression []corev1.NodeSelectorRequirement
	for key, value := range labels {
		matchExpression = append(matchExpression, corev1.NodeSelectorRequirement{
			Key:      key,
			Operator: corev1.NodeSelectorOpIn,
			Values:   []string{value},
		})
	}

	nodeSelector := corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: matchExpression}}}

	// Add the nodeSelector to the pod template.
	deploymentSpec := ldp.Spec.Template.Spec
	deploymentSpec.Template.Spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: nodeSelector.DeepCopy(),
		}}

	deploymentLabels := map[string]string{}
	for k, v := range labels {
		deploymentLabels[k] = v
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    fmt.Sprintf("%s-", ldp.Name),
			Namespace:       ldp.Namespace,
			Labels:          deploymentLabels,
			Annotations:     ldp.Spec.Template.Annotations,
			OwnerReferences: ldp.Spec.Template.OwnerReferences,
			Finalizers:      ldp.Spec.Template.Finalizers,
			ClusterName:     ldp.Spec.Template.ClusterName,
			ManagedFields:   ldp.Spec.Template.ManagedFields,
		},
		Spec: deploymentSpec,
	}

	if err := ctrlutils.SetControllerReference(ldp, deployment, s); err != nil {
		klog.Errorf("%s -> Unable to set the controller reference for a deployment of the LiqoDeployment %s", err, ldp.Name)
		return err, nil
	}

	return nil, deployment
}
