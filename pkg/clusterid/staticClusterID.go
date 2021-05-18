package clusterid

type staticClusterID struct {
	id string
}

// NewStaticClusterID returns a clusterID interface compliant object that stores a read-only clusterID.
func NewStaticClusterID(clusterID string) ClusterID {
	return &staticClusterID{
		id: clusterID,
	}
}

// SetupClusterID function not implemented.
func (staticCID *staticClusterID) SetupClusterID(namespace string) error {
	panic("not implemented")
}

// GetClusterID returns the clusterID string.
func (staticCID *staticClusterID) GetClusterID() string {
	return staticCID.id
}
