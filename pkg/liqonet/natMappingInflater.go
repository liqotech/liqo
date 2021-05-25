package liqonet

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"

	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

// NatMappingInflaterInterface is the interface to be implemented for
// managing NAT mappings for a remote cluster.
type NatMappingInflaterInterface interface {
	// InitNatMappings does everything necessary to set up NAT mappings for a remote cluster.
	InitNatMappings(podCIDR, externalCIDR, clusterID string) error
	// TerminateNatMappings frees/deletes resources allocated for remote cluster.
	TerminateNatMappings(clusterID string) error
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
}

const natMappingPrefix = "natmapping-"

// NewInflater returns a NatMappingInflater istance.
func NewInflater(dynClient dynamic.Interface) *NatMappingInflater {
	return &NatMappingInflater{
		dynClient: dynClient,
	}
}

// InitNatMappings creates a NatMapping resource for the remote cluster.
func (inflater *NatMappingInflater) InitNatMappings(podCIDR, externalCIDR, clusterID string) error {
	// Check existence of resource
	natMappings, err := inflater.getNatMappings(clusterID)
	if err != nil && !errors.IsNotFound(err) {
		// Unknown error
		return fmt.Errorf("cannot retrieve natMapping resource for cluster %s: %w", clusterID, err)
	}
	if err == nil && natMappings != nil {
		// Resource already exists
		return nil
	}
	// Resource does not exist yet
	res := &netv1alpha1.NatMapping{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "net.liqo.io/v1alpha1",
			Kind:       "NatMapping",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: natMappingPrefix,
			Labels: map[string]string{
				"net.liqo.io/natmapping":     "true",
				liqoconst.ClusterIDLabelName: clusterID,
			},
		},
		Spec: netv1alpha1.NatMappingSpec{
			ClusterID:    clusterID,
			PodCIDR:      podCIDR,
			ExternalCIDR: externalCIDR,
			Mappings:     make(map[string]string),
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
		klog.Errorf("cannot create natMapping resource: %s", err.Error())
		return err
	}
	klog.Infof("Resource %s for cluster %s successfully created", up.GetName(), clusterID)
	return nil
}

// TerminateNatMappings deletes the NatMapping resource for remote cluster.
func (inflater *NatMappingInflater) TerminateNatMappings(clusterID string) error {
	// Get resource for remote cluster
	natMappings, err := inflater.getNatMappings(clusterID)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("cannot retrieve natMapping resource for cluster %s: %w", clusterID, err)
	}
	if err != nil && errors.IsNotFound(err) {
		return nil
	}
	// Delete resource
	err = inflater.dynClient.Resource(netv1alpha1.NatMappingGroupResource).Delete(
		context.Background(), natMappings.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	klog.Infof("NatMapping resource for cluster %s deleted", clusterID)
	return nil
}

// AddMapping adds a mapping in the resource related to a remote cluster.
func (inflater *NatMappingInflater) AddMapping(oldIP, newIP, clusterID string) error {
	// Get resource for remote cluster
	natMappings, err := inflater.getNatMappings(clusterID)
	if err != nil {
		return fmt.Errorf("cannot retrieve natMapping resource for cluster %s: %w", clusterID, err)
	}
	// Check if mapping already exists
	if value, exists := natMappings.Spec.Mappings[oldIP]; exists && value == newIP {
		return nil
	}
	// Mapping does not exist yet
	natMappings.Spec.Mappings[oldIP] = newIP
	// Update resource
	if err := inflater.updateNatMappings(natMappings); err != nil {
		return fmt.Errorf("cannot update natMapping resource for cluster %s: %w", clusterID, err)
	}
	return nil
}

// RemoveMapping deletes a mapping from the resource related to a remote cluster.
func (inflater *NatMappingInflater) RemoveMapping(oldIP, clusterID string) error {
	// Get resource for remote cluster
	natMappings, err := inflater.getNatMappings(clusterID)
	if err != nil {
		return fmt.Errorf("cannot retrieve natMapping resource for cluster %s: %w", clusterID, err)
	}
	// Check if mapping exists
	_, exists := natMappings.Spec.Mappings[oldIP]
	if !exists {
		return nil
	}
	// Delete mapping
	delete(natMappings.Spec.Mappings, oldIP)
	// Update
	if err := inflater.updateNatMappings(natMappings); err != nil {
		return fmt.Errorf("cannot update natMapping resource for cluster %s: %w", clusterID, err)
	}
	return nil
}

// Updates the resource related to a remote cluster.
func (inflater *NatMappingInflater) updateNatMappings(resource *netv1alpha1.NatMapping) error {
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
func (inflater *NatMappingInflater) getNatMappings(clusterID string) (*netv1alpha1.NatMapping, error) {
	res := &netv1alpha1.NatMapping{}
	list, err := inflater.dynClient.
		Resource(netv1alpha1.NatMappingGroupResource).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("net.liqo.io/natmapping=true,%s=%s", liqoconst.ClusterIDLabelName, clusterID),
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get natMapping resource for cluster %s: %w", clusterID, err)
	}
	if len(list.Items) != 1 {
		if len(list.Items) != 0 {
			return nil, fmt.Errorf("multiple resources of type %s for the same remote cluster found",
				netv1alpha1.NatMappingGroupResource)
		}
		return nil, errors.NewNotFound(netv1alpha1.NatMappingGroupResource.GroupResource(), "")
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, res)
	if err != nil {
		return nil, fmt.Errorf("cannot map unstructured resource to natMapping resource: %w", err)
	}
	return res, nil
}

// GetNatMappings retrieves resource relative to a remote cluster and returns a slice of NAT mappings.
func (inflater *NatMappingInflater) GetNatMappings(clusterID string) (map[string]string, error) {
	res, err := inflater.getNatMappings(clusterID)
	if err != nil {
		return nil, fmt.Errorf("cannot retrieve natMapping resource for cluster %s: %w", clusterID, err)
	}
	return res.Spec.Mappings, nil
}
