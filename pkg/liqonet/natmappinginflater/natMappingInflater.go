package natmappinginflater

import (
	"context"
	"fmt"

	k8sErr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog"

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
	// GetNatMappings returns the set of mappings related to a remote cluster.
	GetNatMappings(clusterID string) (map[string]string, error)
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
		dynClient:             dynClient,
		natMappingsPerCluster: make(map[string]netv1alpha1.Mappings),
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
	// Check if resource for remote cluster already exists, this can happen if this Pod
	// has been re-scheduled.
	resource, err := inflater.getNatMappingResource(clusterID)
	if err != nil && !k8sErr.IsNotFound(err) {
		return err
	}
	if err == nil {
		inflater.recoverFromResource(resource)
		return nil
	}
	// error was NotFound, therefore resource and in-memory structure have to be created
	// Init natMappingsPerCluster
	inflater.natMappingsPerCluster[clusterID] = make(netv1alpha1.Mappings)
	// Init resource
	return inflater.initResource(podCIDR, externalCIDR, clusterID)
}

func (inflater *NatMappingInflater) recoverFromResource(resource *netv1alpha1.NatMapping) {
	inflater.natMappingsPerCluster[resource.Spec.ClusterID] = resource.Spec.ClusterMappings
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
