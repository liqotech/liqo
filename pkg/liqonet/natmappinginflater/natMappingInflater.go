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

package natmappinginflater

import (
	"context"
	"fmt"

	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqonet/errors"
	"github.com/liqotech/liqo/pkg/liqonet/utils"
)

// Interface is the interface to be implemented for
// managing NAT mappings for a remote cluster.
type Interface interface {
	// InitNatMappingsPerCluster does everything necessary to set up NAT mappings for a remote cluster.
	// podCIDR is the network used for remote pods in the local cluster:
	// it can be either the RemotePodCIDR or the RemoteNATPodCIDR.
	// externalCIDR is the ExternalCIDR used in the remote cluster for local exported resources:
	// it can be either the LocalExternalCIDR or the LocalNATExternalCIDR.
	InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID string) error
	// TerminateNatMappingsPerCluster frees/deletes resources allocated for remote cluster.
	TerminateNatMappingsPerCluster(clusterID string) error
	// GetNatMappings returns the set of mappings related to a remote cluster.
	GetNatMappings(clusterID string) (map[string]string, error)
	// AddMapping adds a NAT mapping.
	AddMapping(oldIP, newIP, clusterID string) error
	// RemoveMapping removes a NAT mapping.
	RemoveMapping(oldIP, clusterID string) error
}

// NatMappingInflater is an implementation of the NatMappingInflaterInterface
// that makes use of a CR, called NatMapping.
type NatMappingInflater struct {
	dynClient dynamic.Interface
	// Set of mappings per cluster. Key is the clusterID, value is the set of mappings for that cluster.
	// This will be used as a backup for the CR.
	natMappingsPerCluster map[string]netv1alpha1.Mappings
}

const (
	natMappingPrefix = "natmapping-"
)

// NewInflater returns a NatMappingInflater istance.
func NewInflater(dynClient dynamic.Interface) *NatMappingInflater {
	inflater := &NatMappingInflater{
		dynClient: dynClient,
	}
	err := inflater.recoverMappingsFromResources()
	if err != nil {
		klog.Error(err)
	}
	err = inflater.ipamConsistencyCheck()
	if err != nil {
		klog.Error(err)
	}
	return inflater
}

func checkParams(podCIDR, externalCIDR, clusterID string) error {
	if podCIDR == "" {
		return &errors.WrongParameter{
			Parameter: "PodCIDR",
			Reason:    errors.StringNotEmpty,
		}
	}
	if externalCIDR == "" {
		return &errors.WrongParameter{
			Parameter: "ExternalCIDR",
			Reason:    errors.StringNotEmpty,
		}
	}
	if clusterID == "" {
		return &errors.WrongParameter{
			Parameter: "ClusterID",
			Reason:    errors.StringNotEmpty,
		}
	}
	if err := utils.IsValidCIDR(podCIDR); err != nil {
		return &errors.WrongParameter{
			Reason:    errors.ValidCIDR,
			Parameter: podCIDR,
		}
	}
	if err := utils.IsValidCIDR(externalCIDR); err != nil {
		return &errors.WrongParameter{
			Reason:    errors.ValidCIDR,
			Parameter: externalCIDR,
		}
	}
	return nil
}

// InitNatMappingsPerCluster creates a NatMapping resource for the remote cluster.
func (inflater *NatMappingInflater) InitNatMappingsPerCluster(podCIDR, externalCIDR, clusterID string) error {
	// Check parameters
	if err := checkParams(podCIDR, externalCIDR, clusterID); err != nil {
		return err
	}
	// Check if it has been already initialized
	if _, exists := inflater.natMappingsPerCluster[clusterID]; exists {
		return nil
	}
	// Init natMappingsPerCluster
	inflater.natMappingsPerCluster[clusterID] = make(netv1alpha1.Mappings)
	// Init resource
	return inflater.initResource(podCIDR, externalCIDR, clusterID)
}

func (inflater *NatMappingInflater) initResource(podCIDR, externalCIDR, clusterID string) error {
	// Check existence of resource
	natMappings, err := inflater.getNatMappingResource(clusterID)
	if err != nil && !k8sErr.IsNotFound(err) {
		// Unknown error
		return fmt.Errorf("cannot retrieve NatMapping resource for cluster %s: %w", clusterID, err)
	}
	if err == nil && natMappings != nil {
		// Resource already exists
		return nil
	}
	// Resource does not exist yet
	res := &netv1alpha1.NatMapping{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "net.liqo.io/v1alpha1",
			Kind:       consts.NatMappingKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: natMappingPrefix,
			Labels: map[string]string{
				consts.NatMappingResourceLabelKey: consts.NatMappingResourceLabelValue,
				consts.ClusterIDLabelName:         clusterID,
			},
		},
		Spec: netv1alpha1.NatMappingSpec{
			ClusterID:       clusterID,
			PodCIDR:         podCIDR,
			ExternalCIDR:    externalCIDR,
			ClusterMappings: make(netv1alpha1.Mappings),
		},
	}
	unstructuredResource, err := runtime.DefaultUnstructuredConverter.ToUnstructured(res)
	if err != nil {
		klog.Errorf("cannot map resource to unstructured resource: %s", err.Error())
		return err
	}
	// Create resource
	up, err := inflater.dynClient.
		Resource(netv1alpha1.NatMappingGroupResource).
		Create(context.Background(), &unstructured.Unstructured{Object: unstructuredResource}, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("cannot create NatMapping resource: %s", err.Error())
		return err
	}
	klog.Infof("Resource %s for cluster %s successfully created", up.GetName(), clusterID)
	return nil
}

// TerminateNatMappingsPerCluster deletes the NatMapping resource for remote cluster.
func (inflater *NatMappingInflater) TerminateNatMappingsPerCluster(clusterID string) error {
	if err := inflater.deleteResourceForCluster(clusterID); err != nil {
		return fmt.Errorf("unable to delete resource for cluster %s: %w", clusterID, err)
	}
	// Remove entry in natMappingsPerCluster
	delete(inflater.natMappingsPerCluster, clusterID)
	return nil
}

// Function that deletes the resource NatMapping for a specific remote cluster.
// It carries out multiple tentatives until it manages to delete the resource.
func (inflater *NatMappingInflater) deleteResourceForCluster(clusterID string) error {
	retryError := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Get resource for remote cluster
		natMappings, err := inflater.getNatMappingResource(clusterID)
		if err != nil && !k8sErr.IsNotFound(err) {
			return fmt.Errorf("cannot retrieve NatMapping resource for cluster %s: %w", clusterID, err)
		}
		if err != nil && k8sErr.IsNotFound(err) {
			return nil
		}
		// Remove labels before deleting resource is necessary
		// because otherwise the informer will be triggered and will
		// re-create the resource.
		delete(natMappings.ObjectMeta.Labels, consts.NatMappingResourceLabelKey)
		if err := inflater.updateNatMappingResource(natMappings); err != nil {
			return fmt.Errorf("cannot update NatMapping resource for cluster %s: %w", clusterID, err)
		}
		// Delete resource
		err = inflater.dynClient.Resource(netv1alpha1.NatMappingGroupResource).Delete(
			context.Background(), natMappings.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		klog.Infof("NatMapping resource for cluster %s deleted", clusterID)
		return nil
	})
	if retryError != nil {
		return retryError
	}
	return nil
}

// AddMapping adds a mapping in the resource related to a remote cluster.
// It also adds the mapping in natMappingsPerCluster.
func (inflater *NatMappingInflater) AddMapping(oldIP, newIP, clusterID string) error {
	var exists bool
	var mappings netv1alpha1.Mappings
	// Check if NAT mappings have been initilized for remote cluster.
	mappings, exists = inflater.natMappingsPerCluster[clusterID]
	if !exists {
		return &errors.MissingInit{
			StructureName: fmt.Sprintf("%s for cluster %s", consts.NatMappingKind, clusterID),
		}
	}
	// Check existence of mapping
	existingIP, exists := mappings[oldIP]
	if exists && existingIP == newIP {
		return nil // Mapping already exists, do nothing
	}
	// Add/Update mapping in memory structure
	mappings[oldIP] = newIP
	if err := inflater.addOrUpdateMappingInResource(oldIP, newIP, clusterID); err != nil {
		delete(mappings, oldIP) // Undo add
		return fmt.Errorf("unable to add NatMapping to resource: %w", err)
	}
	return nil
}

func (inflater *NatMappingInflater) addOrUpdateMappingInResource(oldIP, newIP, clusterID string) error {
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Get resource for remote cluster
		natMappings, err := inflater.getNatMappingResource(clusterID)
		if err != nil {
			return fmt.Errorf("cannot retrieve NatMapping resource for cluster %s: %w", clusterID, err)
		}
		natMappings.Spec.ClusterMappings[oldIP] = newIP
		// Update resource
		if err := inflater.updateNatMappingResource(natMappings); err != nil {
			return fmt.Errorf("cannot update NatMapping resource for cluster %s: %w", clusterID, err)
		}
		if err != nil {
			return err
		}
		return nil
	})
	if retryError != nil {
		return retryError
	}
	return nil
}

// RemoveMapping removes a mapping from both resource and in-memory structure.
func (inflater *NatMappingInflater) RemoveMapping(oldIP, clusterID string) error {
	var exists bool
	var mappings netv1alpha1.Mappings
	// Check if NAT mappings have been initialized for remote cluster.
	mappings, exists = inflater.natMappingsPerCluster[clusterID]
	if !exists {
		return &errors.MissingInit{
			StructureName: fmt.Sprintf("%s for cluster %s", consts.NatMappingKind, clusterID),
		}
	}
	// Check existence of mapping
	newIP, exists := mappings[oldIP]
	if !exists {
		return nil // Mapping already deleted, do nothing
	}
	// Delete mapping from in-memory structure.
	delete(mappings, oldIP)
	if err := inflater.removeMappingFromResource(oldIP, clusterID); err != nil {
		mappings[oldIP] = newIP // Undo delete
		return fmt.Errorf("cannot delete mapping from resource: %w", err)
	}
	return nil
}

// removeMapping deletes a mapping from the resource related to a remote cluster.
func (inflater *NatMappingInflater) removeMappingFromResource(oldIP, clusterID string) error {
	retryError := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Get resource for remote cluster
		natMappings, err := inflater.getNatMappingResource(clusterID)
		if err != nil {
			return fmt.Errorf("cannot retrieve NatMapping resource for cluster %s: %w", clusterID, err)
		}
		// Delete mapping
		delete(natMappings.Spec.ClusterMappings, oldIP)
		// Update
		if err := inflater.updateNatMappingResource(natMappings); err != nil {
			return fmt.Errorf("cannot update NatMapping resource for cluster %s: %w", clusterID, err)
		}
		return nil
	})
	if retryError != nil {
		return retryError
	}
	return nil
}

// Updates the resource related to a remote cluster.
func (inflater *NatMappingInflater) updateNatMappingResource(resource *netv1alpha1.NatMapping) error {
	// Convert resource to unstructured type
	unstructuredResource, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resource)
	if err != nil {
		klog.Errorf("cannot map resource to unstructured resource: %s", err.Error())
		return err
	}

	// Update
	_, err = inflater.dynClient.Resource(netv1alpha1.NatMappingGroupResource).Update(context.Background(),
		&unstructured.Unstructured{Object: unstructuredResource}, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

// Retrieve resource relative to a remote cluster.
func (inflater *NatMappingInflater) getNatMappingResource(clusterID string) (*netv1alpha1.NatMapping, error) {
	var res unstructured.Unstructured
	nm := &netv1alpha1.NatMapping{}
	list, err := inflater.dynClient.
		Resource(netv1alpha1.NatMappingGroupResource).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s,%s=%s",
				consts.NatMappingResourceLabelKey,
				consts.NatMappingResourceLabelValue,
				consts.ClusterIDLabelName, clusterID),
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get NatMapping resource for cluster %s: %w", clusterID, err)
	}
	if len(list.Items) != 1 {
		if len(list.Items) != 0 {
			res, err = inflater.deleteMultipleNatMappingResources(list.Items)
			if err != nil {
				return nil, fmt.Errorf("cannot delete multiple NatMapping resources: %w", err)
			}
		} else {
			return nil, k8sErr.NewNotFound(netv1alpha1.NatMappingGroupResource.GroupResource(), "")
		}
	}
	res = list.Items[0]
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.Object, nm)
	if err != nil {
		return nil, fmt.Errorf("cannot map unstructured resource to NatMapping resource: %w", err)
	}
	return nm, nil
}

// GetNatMappings returns the set of NAT mappings related to a remote cluster.
func (inflater *NatMappingInflater) GetNatMappings(clusterID string) (map[string]string, error) {
	// Check if NAT mappings have been initilized for remote cluster.
	mappings, exists := inflater.natMappingsPerCluster[clusterID]
	if !exists {
		return nil, &errors.MissingInit{
			StructureName: fmt.Sprintf("%s for cluster %s", consts.NatMappingKind, clusterID),
		}
	}
	// If execution reached this point, this means initialization
	// had been carried out for remote cluster.
	return mappings, nil
}

// Function that keeps a resource and removes remaining ones in case multiple resources exist.
// Return value is the survived resource.
func (inflater *NatMappingInflater) deleteMultipleNatMappingResources(resources []unstructured.Unstructured) (unstructured.Unstructured, error) {
	// Keep last resource of the slice
	survived := resources[len(resources)-1]
	resources = resources[:len(resources)-1]
	for _, res := range resources {
		// First remove Liqo label of resources so that informer is not triggered
		err := unstructured.SetNestedMap(res.Object, make(map[string]interface{}), "metadata", "labels")
		if err != nil {
			return unstructured.Unstructured{}, fmt.Errorf("cannot remove labels to NatMapping resource: %w", err)
		}
		// Delete resource
		err = inflater.dynClient.Resource(netv1alpha1.NatMappingGroupResource).Delete(context.Background(),
			res.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return unstructured.Unstructured{}, fmt.Errorf("cannot delete NatMapping resource: %w", err)
		}
	}
	return survived, nil
}

func (inflater *NatMappingInflater) recoverMappingsFromResources() error {
	nm := &netv1alpha1.NatMapping{}
	list, err := inflater.dynClient.
		Resource(netv1alpha1.NatMappingGroupResource).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s",
				consts.NatMappingResourceLabelKey,
				consts.NatMappingResourceLabelValue),
		})
	if err != nil {
		return fmt.Errorf("unable to get NatMapping resources: %w", err)
	}
	// Allocate a map with capacity equal to the number of found resources
	inflater.natMappingsPerCluster = make(map[string]netv1alpha1.Mappings, len(list.Items))
	for _, unstructuredMapping := range list.Items {
		// For each resource, convert and store it in the map.
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredMapping.Object, nm)
		if err != nil {
			return fmt.Errorf("cannot map unstructured resource to NatMapping resource: %w", err)
		}
		inflater.natMappingsPerCluster[nm.Spec.ClusterID] = nm.Spec.ClusterMappings.DeepCopy()
		klog.Infof("Mappings for cluster %s has been recovered from natmapping resource", nm.Spec.ClusterID)
	}
	klog.Infof("In memory structure of NatMappingInflater has been successfully recovered from resources.")
	return nil
}

func (inflater *NatMappingInflater) ipamConsistencyCheck() error {
	ipamConfig, err := inflater.getIPAMConfig()
	if err != nil {
		return err
	}
	klog.Infof("Consistency check between IPAM configuration and NatMapping resources..")
	// For each mapping in the IPAM storage, check if there is a corresponding NatMapping
	for oldIP, mapping := range ipamConfig.Spec.EndpointMappings {
		for clusterID := range mapping.ClusterMappings {
			if inflater.natMappingsPerCluster[clusterID][oldIP] != mapping.IP {
				if err := inflater.AddMapping(oldIP, mapping.IP, clusterID); err != nil {
					return err
				}
				klog.Infof("Mapping of %s on %s of cluster %s has been recovered from IPAM configuration",
					oldIP, mapping.IP, clusterID)
			}
		}
	}
	// For each NatMapping, check if it exists on IPAM. If not, delete the NatMapping.
	for clusterID, natMapping := range inflater.natMappingsPerCluster {
		for oldIP, newIP := range natMapping {
			ipamMapping, endpointMappingExists := ipamConfig.Spec.EndpointMappings[oldIP]
			if !endpointMappingExists {
				err = inflater.RemoveMapping(oldIP, clusterID)
				if err != nil {
					return err
				}
				klog.Infof("Mapping of %s on %s for cluster %s has been removed"+
					"because it has not been found in IPAM configuration", oldIP, newIP, clusterID)
				continue
			}
			_, mappingIsActiveOnCluster := ipamMapping.ClusterMappings[clusterID]
			if !mappingIsActiveOnCluster {
				err = inflater.RemoveMapping(oldIP, clusterID)
				if err != nil {
					return err
				}
				klog.Infof("Mapping of %s on %s for cluster %s has been removed"+
					"because it has not been found in IPAM configuration", oldIP, newIP, clusterID)
			}
		}
	}
	klog.Infof("IPAM configuration and NatMapping resources are now aligned.")
	return nil
}

func (inflater *NatMappingInflater) getIPAMConfig() (*netv1alpha1.IpamStorage, error) {
	res := &netv1alpha1.IpamStorage{}
	list, err := inflater.dynClient.
		Resource(netv1alpha1.IpamGroupVersionResource).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", consts.IpamStorageResourceLabelKey, consts.IpamStorageResourceLabelValue),
		})
	if err != nil {
		klog.Errorf(err.Error())
		return nil, fmt.Errorf("unable to get IPAM configuration: %w", err)
	}
	if len(list.Items) != 1 {
		if len(list.Items) != 0 {
			return nil, fmt.Errorf("multiple resources of type %s found", netv1alpha1.IpamGroupVersionResource)
		}
		return nil, k8sErr.NewNotFound(netv1alpha1.IpamGroupVersionResource.GroupResource(), "")
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, res)
	if err != nil {
		return nil, fmt.Errorf("cannot map unstructured resource to ipam storage resource:%w", err)
	}
	return res, nil
}
