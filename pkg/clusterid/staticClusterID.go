package clusterid

type staticClusterID struct {
	id string
}

func NewStaticClusterID(clusterID string) ClusterID {
	return &staticClusterID{
		id: clusterID,
	}
}

func (staticCID *staticClusterID) SetupClusterID(namespace string) error {
	panic("not implemented")
}

func (staticCID *staticClusterID) GetClusterID() string {
	return staticCID.id
}
