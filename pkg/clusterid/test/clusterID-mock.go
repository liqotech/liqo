package test

// ClusterIDMock implements a mock for the ClusterID type.
type ClusterIDMock struct {
	Id string
}

// SetupClusterID sets a new clusterid.
func (cId *ClusterIDMock) SetupClusterID(namespace string) error {
	cId.Id = "local-cluster"
	return nil
}

// GetClusterID retrieves the clusterid.
func (cId *ClusterIDMock) GetClusterID() string {
	return cId.Id
}
