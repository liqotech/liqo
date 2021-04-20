package test

type ClusterIDMock struct {
	Id string
}

func (cId *ClusterIDMock) SetupClusterID(namespace string) error {
	cId.Id = "local-cluster"
	return nil
}

func (cId *ClusterIDMock) GetClusterID() string {
	return cId.Id
}
