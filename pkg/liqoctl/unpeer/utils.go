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

package unpeer

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

func deleteResourceSlicesByClusterID(ctx context.Context, f *factory.Factory,
	remoteClusterID liqov1beta1.ClusterID, waitForActualDeletion bool, forceClusterId string) error {
	s := f.Printer.StartSpinner("Deleting resourceslices")

	rsSelector := labels.Set{
		consts.ReplicationRequestedLabel:   consts.ReplicationRequestedLabelValue,
		consts.ReplicationDestinationLabel: string(remoteClusterID),
	}

	resourceSlices, err := getters.ListResourceSlicesByLabel(ctx, f.CRClient,
		corev1.NamespaceAll, labels.SelectorFromSet(rsSelector))

	switch {
	case err != nil:
		s.Fail("Error while retrieving resourceslices: ", output.PrettyErr(err))
		return err
	case len(resourceSlices) == 0:
		s.Success("No resourceslices found")
	default:

		for i := range resourceSlices {
			if err := client.IgnoreNotFound(f.CRClient.Delete(ctx, &resourceSlices[i])); err != nil {
				s.Fail("Error while deleting resourceslice ", client.ObjectKeyFromObject(&resourceSlices[i]), ": ", output.PrettyErr(err))
				return err
			}
		}

		s.Success("All resourceslices deleted")

		if waitForActualDeletion {
			// wait for all resourceslices to be deleted
			waiter := wait.NewWaiterFromFactory(f)
			err := waiter.ForResourceSlicesAbsence(ctx, corev1.NamespaceAll, labels.SelectorFromSet(rsSelector))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// deleteVirtualNodesByClusterID removes all virtual nodes associated with a specific remote cluster.
func deleteVirtualNodesByClusterID(ctx context.Context, f *factory.Factory,
	remoteClusterID liqov1beta1.ClusterID, waitForActualDeletion bool) error {
	s := f.Printer.StartSpinner("Deleting virtualnodes")

	virtualNodes, err := getters.ListVirtualNodesByClusterID(ctx, f.CRClient, remoteClusterID)

	switch {
	case err != nil:
		s.Fail("Error while retrieving virtualnodes: ", output.PrettyErr(err))
		return err
	case len(virtualNodes) == 0:
		s.Success("No virtualnodes found")
	default:
		for i := range virtualNodes {
			if err := client.IgnoreNotFound(f.CRClient.Delete(ctx, &virtualNodes[i])); err != nil {
				s.Fail("Error while deleting virtualnode ", client.ObjectKeyFromObject(&virtualNodes[i]), ": ", output.PrettyErr(err))
				return err
			}
		}
		s.Success("All virtualnodes deleted")

		if waitForActualDeletion {
			// wait for all virtualnodes to be deleted
			waiter := wait.NewWaiterFromFactory(f)
			if err := waiter.ForVirtualNodesAbsence(ctx, remoteClusterID); err != nil {
				return err
			}
		}
	}

	return nil
}

// Adds the force-unpeer annotation to all Liqo resources belonging to the specified remote cluster and then deletes them.
// This function is used during forced unpeering operations to ensure proper cleanup of NamespaceMap and NamespaceOffloading resources.
func ForceAnnotateAndDeleteAllLiqoResources(ctx context.Context, f *factory.Factory, remoteClusterID liqov1beta1.ClusterID) error {
	// NamespaceMap
	nsMapList := &offv1beta1.NamespaceMapList{}
	if err := f.CRClient.List(ctx, nsMapList, &client.ListOptions{}); err == nil {
		for i := range nsMapList.Items {
			nsMap := &nsMapList.Items[i]
			if nsMap.Labels[consts.RemoteClusterID] == string(remoteClusterID) {
				patch := client.MergeFrom(nsMap.DeepCopy())
				if nsMap.Annotations == nil {
					nsMap.Annotations = map[string]string{}
				}
				nsMap.Annotations["liqo.io/force-unpeer"] = "true"
				_ = f.CRClient.Patch(ctx, nsMap, patch)
				_ = f.CRClient.Delete(ctx, nsMap)
			}
		}
	}

	// NamespaceOffloading
	nsOffList := &offv1beta1.NamespaceOffloadingList{}
	if err := f.CRClient.List(ctx, nsOffList, &client.ListOptions{}); err == nil {
		for i := range nsOffList.Items {
			nsOff := &nsOffList.Items[i]
			if nsOff.Labels[consts.RemoteClusterID] == string(remoteClusterID) {
				patch := client.MergeFrom(nsOff.DeepCopy())
				if nsOff.Annotations == nil {
					nsOff.Annotations = map[string]string{}
				}
				nsOff.Annotations["liqo.io/force-unpeer"] = "true"
				_ = f.CRClient.Patch(ctx, nsOff, patch)
				_ = f.CRClient.Delete(ctx, nsOff)
			}
		}
	}

	return nil
}
