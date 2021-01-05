package test

type ClusterIDMock struct {
	id string
}

func (cId *ClusterIDMock) SetupClusterID(namespace string) error {
	cId.id = "local-cluster"
	return nil
}

func (cId *ClusterIDMock) GetClusterID() string {
	return cId.id
}
