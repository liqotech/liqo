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

package netcfgcreator

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/liqonet/tunnel/wireguard"
)

const networkConfigNamePrefix = "net-config-"

// GetLocalNetworkConfig returns the local NetworkConfig associated with a given cluster ID.
// In case more than one NetworkConfig is found, all but the oldest are deleted.
func GetLocalNetworkConfig(ctx context.Context, c client.Client, clusterID, namespace string) (*netv1alpha1.NetworkConfig, error) {
	networkConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdreplicator.DestinationLabel: clusterID}

	if err := c.List(ctx, networkConfigList, labels, client.InNamespace(namespace)); err != nil {
		klog.Errorf("An error occurred while listing NetworkConfigs: %v", err)
		return nil, err
	}

	switch len(networkConfigList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(netv1alpha1.NetworkConfigGroupResource,
			fmt.Sprintf("Local NetworkConfig for cluster: %v", clusterID))
	case 1:
		return &networkConfigList.Items[0], nil
	default:
		// Multiple NetworkConfigs for the same cluster have been created, due to a race condition with the informers cache.
		klog.V(4).Infof("Found multiple instances of local NetworkConfigs for remote cluster %v", clusterID)

		netcfg, duplicates := filterDuplicateNetworkConfig(networkConfigList.Items)
		for i := range duplicates {
			if err := c.Delete(ctx, &duplicates[i]); client.IgnoreNotFound(err) != nil {
				klog.Errorf("An error occurred while deleting duplicate NetworkConfig %q: %v", klog.KObj(&duplicates[i]), err)
				return nil, err
			}
			klog.V(4).Infof("Successfully deleted duplicate NetworkConfig %q for remote cluster %v", klog.KObj(&duplicates[i]), clusterID)
		}

		return netcfg, nil
	}
}

// GetRemoteNetworkConfig returns the remote NetworkConfig associated with a given cluster ID.
func GetRemoteNetworkConfig(ctx context.Context, c client.Client, clusterID, namespace string) (*netv1alpha1.NetworkConfig, error) {
	networkConfigList := &netv1alpha1.NetworkConfigList{}
	labels := client.MatchingLabels{crdreplicator.RemoteLabelSelector: clusterID}

	if err := c.List(ctx, networkConfigList, labels, client.InNamespace(namespace)); err != nil {
		klog.Errorf("An error occurred while listing NetworkConfigs: %v", err)
		return nil, err
	}

	switch len(networkConfigList.Items) {
	case 0:
		return nil, kerrors.NewNotFound(netv1alpha1.NetworkConfigGroupResource,
			fmt.Sprintf("Remote NetworkConfig for cluster: %v", clusterID))
	case 1:
		return &networkConfigList.Items[0], nil
	default:
		// Multiple NetworkConfigs for the same cluster have been detected.
		return nil, fmt.Errorf("found multiple instances of remote NetworkConfigs for remote cluster %v", clusterID)
	}
}

// filterDuplicateNetworkConfig filters a list of NetworkConfigs, and selects the duplicated to be deleted.
func filterDuplicateNetworkConfig(items []netv1alpha1.NetworkConfig) (netcfg *netv1alpha1.NetworkConfig, duplicates []netv1alpha1.NetworkConfig) {
	// Sort the elements by creation timestamp and, if equal, by UID.
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreationTimestamp.Before(&items[j].CreationTimestamp) ||
			items[i].GetUID() < items[j].GetUID()
	})

	// Keep the first element (i.e. the oldest one), and mark the others for deletion.
	return &items[0], items[1:]
}

// EnforceNetworkConfigPresence ensures the presence of a local NetworkConfig associated with the given ForeignCluster.
func (ncc *NetworkConfigCreator) EnforceNetworkConfigPresence(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID

	// Check if the resource for the remote cluster already exists
	netcfg, err := GetLocalNetworkConfig(ctx, ncc.Client, clusterID, fc.Status.TenantNamespace.Local)
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	// Create the resource if not already present (if the error is not nil, then at this point is a not found one)
	if err != nil {
		return ncc.createNetworkConfig(ctx, fc)
	}

	// Otherwise, update the resource to ensure it is up-to-date
	return ncc.updateNetworkConfig(ctx, netcfg, fc)
}

// createNetworkConfig creates a new local NetworkConfig associated with the given ForeignCluster.
func (ncc *NetworkConfigCreator) createNetworkConfig(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	netcfg := netv1alpha1.NetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: networkConfigNamePrefix,
			Namespace:    fc.Status.TenantNamespace.Local,
		},
	}
	utilruntime.Must(ncc.populateNetworkConfig(&netcfg, fc))

	if err := ncc.Create(ctx, &netcfg); err != nil {
		klog.Errorf("An error occurred while creating NetworkConfig: %v", err)
		return err
	}
	klog.Infof("NetworkConfig %q successfully created", klog.KObj(&netcfg))
	return nil
}

// updateNetworkConfig ensures the local NetworkConfig associated with the given ForeignCluster is up-to-date.
func (ncc *NetworkConfigCreator) updateNetworkConfig(ctx context.Context, netcfg *netv1alpha1.NetworkConfig,
	fc *discoveryv1alpha1.ForeignCluster) error {
	original := netcfg.DeepCopy()

	if err := ncc.populateNetworkConfig(netcfg, fc); err != nil {
		klog.Errorf("An error occurred while updating NetworkConfig %q: %v", klog.KObj(netcfg), err)
		return err
	}

	if reflect.DeepEqual(original, netcfg) {
		klog.V(4).Infof("NetworkConfig %q already up-to-date", klog.KObj(netcfg))
		return nil
	}

	if err := ncc.Update(ctx, netcfg); err != nil {
		klog.Errorf("An error occurred while updating NetworkConfig %q: %v", klog.KObj(netcfg), err)
		return err
	}
	klog.Infof("NetworkConfig %q successfully updated", klog.KObj(netcfg))
	return nil
}

// populateNetworkConfig sets the correct parameters of the NetworkConfig.
func (ncc *NetworkConfigCreator) populateNetworkConfig(netcfg *netv1alpha1.NetworkConfig, fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID

	if netcfg.Labels == nil {
		netcfg.Labels = map[string]string{}
	}
	netcfg.Labels[crdreplicator.LocalLabelSelector] = strconv.FormatBool(true)
	netcfg.Labels[crdreplicator.DestinationLabel] = clusterID

	wgEndpointIP, wgEndpointPort := ncc.serviceWatcher.WiregardEndpoint()

	netcfg.Spec.ClusterID = clusterID
	netcfg.Spec.PodCIDR = ncc.PodCIDR
	netcfg.Spec.ExternalCIDR = ncc.ExternalCIDR
	netcfg.Spec.EndpointIP = wgEndpointIP
	netcfg.Spec.BackendType = wireguard.DriverName

	if netcfg.Spec.BackendConfig == nil {
		netcfg.Spec.BackendConfig = map[string]string{}
	}
	netcfg.Spec.BackendConfig[wireguard.PublicKey] = ncc.secretWatcher.WiregardPublicKey()
	netcfg.Spec.BackendConfig[wireguard.ListeningPort] = wgEndpointPort

	return controllerutil.SetControllerReference(fc, netcfg, ncc.Scheme)
}

// EnforceNetworkConfigAbsence ensures the absence of local NetworkConfigs associated with the given ForeignCluster.
func (ncc *NetworkConfigCreator) EnforceNetworkConfigAbsence(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	clusterID := fc.Spec.ClusterIdentity.ClusterID
	labels := client.MatchingLabels{crdreplicator.DestinationLabel: clusterID}

	// Let perform a cached list first, to prevent unnecessary interactions with the API server.
	var networkConfigList netv1alpha1.NetworkConfigList
	if err := ncc.List(ctx, &networkConfigList, labels, client.InNamespace(fc.Status.TenantNamespace.Local)); err != nil {
		klog.Errorf("An error occurred while listing NetworkConfigs: %v", err)
		return err
	}

	if len(networkConfigList.Items) == 0 {
		klog.V(4).Infof("No NetworkConfigs associated with cluster ID %q to be removed", clusterID)
		return nil
	}

	var netcfg netv1alpha1.NetworkConfig
	if err := ncc.DeleteAllOf(ctx, &netcfg, labels, client.InNamespace(fc.Status.TenantNamespace.Local)); err != nil {
		klog.Errorf("Failed to remove NetworkConfigs associated with cluster ID %q: %v", clusterID, err)
		return err
	}

	klog.Errorf("Correctly ensured no NetworkConfigs associated with cluster ID %q are present", clusterID)
	return nil
}
