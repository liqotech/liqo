package interfaces

import (
	corev1 "k8s.io/api/core/v1"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
)

// ClusterResourceInterface represents a generic subset of Broadcaster exported methods to be used instead of a direct access to
// the Broadcaster instance and get/update some cluster resources information.
type ClusterResourceInterface interface {
	// ReadResources returns all free cluster resources calculated for a given clusterID scaled by a percentage value.
	ReadResources(clusterID string) corev1.ResourceList
	// RemoveClusterID removes given clusterID from all internal structures and it will be no more valid.
	RemoveClusterID(clusterID string)
	// GetConfig returns a ClusterConfig instance.
	GetConfig() *configv1alpha1.ClusterConfig
}
