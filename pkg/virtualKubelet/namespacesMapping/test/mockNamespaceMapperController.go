package test

import "github.com/liqotech/liqo/pkg/virtualKubelet/namespacesMapping"

type MockNamespaceMapperController struct {
	Mapper *MockNamespaceMapper
}

func NewMockNamespaceMapperController(mapper *MockNamespaceMapper) namespacesMapping.MapperController {
	controller := &MockNamespaceMapperController{
		Mapper: mapper,
	}

	return controller
}

func (c *MockNamespaceMapperController) PollStartOutgoingReflection() chan string {
	panic("to implement")
}

func (c *MockNamespaceMapperController) PollStartIncomingReflection() chan string {
	panic("to implement")
}

func (c *MockNamespaceMapperController) PollStopOutgoingReflection() chan string {
	panic("to implement")
}

func (c *MockNamespaceMapperController) PollStopIncomingReflection() chan string {
	panic("to implement")
}

func (c *MockNamespaceMapperController) PollStartMapper() chan struct{} {
	panic("to implement")
}

func (c *MockNamespaceMapperController) PollStopMapper() chan struct{} {
	panic("to implement")
}

func (c *MockNamespaceMapperController) ReadyForRestart() {
	panic("to implement")
}

func (c *MockNamespaceMapperController) NatNamespace(namespace string, create bool) (string, error) {
	return c.Mapper.NatNamespace(namespace, create)
}

func (c *MockNamespaceMapperController) DeNatNamespace(namespace string) (string, error) {
	return c.Mapper.DeNatNamespace(namespace)
}

func (c *MockNamespaceMapperController) MappedNamespaces() map[string]string {
	panic("implement me")
}

func (c *MockNamespaceMapperController) WaitForSync() {
	panic("implement me")
}
