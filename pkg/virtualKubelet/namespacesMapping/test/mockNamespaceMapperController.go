package test

type MockNamespaceMapperController struct {
	mapper *MockNamespaceMapper
}

func NewMockNamespaceMapperController() *MockNamespaceMapperController {
	controller := &MockNamespaceMapperController{
		mapper: &MockNamespaceMapper{
			cache: map[string]string{},
		},
	}
	return controller
}

func (c *MockNamespaceMapperController) PollStartOutgoingReflection() chan string {
	return make(chan string, 1)
}

func (c *MockNamespaceMapperController) PollStartIncomingReflection() chan string {
	return make(chan string, 1)
}

func (c *MockNamespaceMapperController) PollStopOutgoingReflection() chan string {
	return make(chan string, 1)
}

func (c *MockNamespaceMapperController) PollStopIncomingReflection() chan string {
	return make(chan string, 1)
}

func (c *MockNamespaceMapperController) PollStartMapper() chan struct{} {
	return make(chan struct{}, 1)
}

func (c *MockNamespaceMapperController) PollStopMapper() chan struct{} {
	return make(chan struct{}, 1)
}

func (c *MockNamespaceMapperController) ReadyForRestart() {
}

func (c *MockNamespaceMapperController) NatNamespace(namespace string, create bool) (string, error) {
	return c.mapper.NatNamespace(namespace, create)
}

func (c *MockNamespaceMapperController) DeNatNamespace(namespace string) (string, error) {
	return c.mapper.DeNatNamespace(namespace)
}
