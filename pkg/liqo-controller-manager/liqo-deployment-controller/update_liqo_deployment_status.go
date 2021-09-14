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
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
)

// updateGeneratedDeploymentStatus updates the LiqoDeployment status with information
// about the generation labels of a specific deployment replica.
func updateGeneratedDeploymentStatus(ldp *offv1alpha1.LiqoDeployment, d *appsv1.Deployment, labels map[string]string) {
	if len(ldp.Status.CurrentDeployment) == 0 {
		ldp.Status.CurrentDeployment = map[string]offv1alpha1.GeneratedDeploymentStatus{}
	}
	generationLabels := map[string]string{}
	for k, v := range labels {
		generationLabels[k] = v
	}
	ldp.Status.CurrentDeployment[d.Name] = offv1alpha1.GeneratedDeploymentStatus{
		GenerationLabelsValues: generationLabels,
	}
}

// searchUnnecessaryDeploymentReplicas searches for unnecessary deployment replicas, looking at the generation labels
// specified in the LiqoDeployment status.
func (r *LiqoDeploymentReconciler) searchUnnecessaryDeploymentReplicas(ctx context.Context,
	ldp *offv1alpha1.LiqoDeployment, clusterMap map[string]struct{}) bool {
	deletionNotCompleted := false
	for deploymentName, deploymentInfo := range ldp.Status.CurrentDeployment {
		mapKey := ""
		deployment := &appsv1.Deployment{}

		if err := r.Get(ctx, types.NamespacedName{Namespace: ldp.Namespace, Name: deploymentName}, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				klog.Infof("There is no Deployment %s inside the namespace %s. The corresponding "+
					"entry in the LiqoDeployment %s is deleted", deploymentName, ldp.Namespace, ldp.Name)
				delete(ldp.Status.CurrentDeployment, deploymentName)
				continue
			}
			klog.Errorf("%s -> Unable to get the deployment %s inside the namespace %s.",
				err, deploymentName, ldp.Namespace)
			deletionNotCompleted = true
			continue
		}

		keys := make([]string, 0, len(deploymentInfo.GenerationLabelsValues))
		for key := range deploymentInfo.GenerationLabelsValues {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for i := range keys {
			mapKey = fmt.Sprintf("%s%s%s%s%s", mapKey, labelSeparator, keys[i],
				keyValueSeparator, deploymentInfo.GenerationLabelsValues[keys[i]])
		}

		if _, ok := clusterMap[mapKey]; ok && deployment.DeletionTimestamp.IsZero() {
			if len(deployment.Status.Conditions) > 0 {
				ldp.Status.CurrentDeployment[deploymentName] = offv1alpha1.GeneratedDeploymentStatus{
					DeploymentLastCondition: deployment.Status.Conditions[0],
					GenerationLabelsValues:  deploymentInfo.GenerationLabelsValues,
				}
			}
			// The entry is deleted in case of more than one deployment replicas for a specific combination.
			delete(clusterMap, mapKey)
			continue
		}

		ensureDeploymentDeletion(ctx, r.Client, deployment)
		deletionNotCompleted = true
	}
	return deletionNotCompleted
}

// ensureDeploymentDeletion sets the deletion timestamp on resources that need to be deleted.
func ensureDeploymentDeletion(ctx context.Context, c client.Client, dp *appsv1.Deployment) {
	if dp.DeletionTimestamp.IsZero() {
		if err := c.Delete(ctx, dp); err != nil {
			klog.Errorf("%s -> Unable to set the deletion timestamp on the deployment %s "+
				"inside the namespace %s.", err, dp.Name, dp.Namespace)
		}
	}
	klog.Infof("waiting for the deployment '%s' inside the namespace '%s' to be deleted", dp.Name, dp.Namespace)
}
