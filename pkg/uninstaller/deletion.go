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

package uninstaller

import (
	"context"
	"encoding/json"

	"gomodules.xyz/jsonpatch/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// UnjoinClusters disables incoming and outgoing peerings with available clusters.
func UnjoinClusters(ctx context.Context, client dynamic.Interface) error {
	foreign, err := getForeignList(client)
	if err != nil {
		return err
	}
	klog.Infof("Unjoin %v ForeignClusters", len(foreign.Items))

	mutation := func(fc *discoveryV1alpha1.ForeignCluster) {
		fc.Spec.IncomingPeeringEnabled = discoveryV1alpha1.PeeringEnabledNo
		fc.Spec.OutgoingPeeringEnabled = discoveryV1alpha1.PeeringEnabledNo
	}

	for index := range foreign.Items {
		if err := patchForeignCluster(ctx, mutation, &foreign.Items[index], client); err != nil {
			return err
		}
	}
	return nil
}

// patchForeignCluster patches the given foreign cluster applying the provided function.
func patchForeignCluster(ctx context.Context, changeFunc func(*discoveryV1alpha1.ForeignCluster),
	foreignCluster *discoveryV1alpha1.ForeignCluster, client dynamic.Interface) error {
	original, err := json.Marshal(foreignCluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	mutated := foreignCluster.DeepCopy()
	changeFunc(mutated)

	target, err := json.Marshal(mutated)
	if err != nil {
		klog.Error(err)
		return err
	}

	ops, err := jsonpatch.CreatePatch(original, target)
	if err != nil {
		klog.Error(err)
		return err
	}

	if len(ops) == 0 {
		// this avoids an empty patch of the foreign cluster
		return nil
	}

	bytes, err := json.Marshal(ops)
	if err != nil {
		klog.Error(err)
		return err
	}

	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	_, err = r1.Patch(ctx, mutated.Name, types.JSONPatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

// DeleteAllForeignClusters deletes all ForeignCluster resources.
func DeleteAllForeignClusters(ctx context.Context, client dynamic.Interface) error {
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	err := r1.DeleteCollection(ctx,
		metav1.DeleteOptions{TypeMeta: metav1.TypeMeta{}}, metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	return err
}
