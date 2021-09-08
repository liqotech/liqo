// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
)

// MockNamespaceMapperController implements a mock namespace mapper controller.
type MockNamespaceMapperController struct {
	Mapper *MockNamespaceMapper
}

// NewMockNamespaceMapperController builds and returns a new mock NewNamespaceMapperController.
func NewMockNamespaceMapperController(mapper *MockNamespaceMapper) namespacesmapping.MapperController {
	controller := &MockNamespaceMapperController{
		Mapper: mapper,
	}

	return controller
}

// PollStartOutgoingReflection returns the startOutgoingReflection channel. It is used to receive elements to start new outgoing reflection
// routines on pushed namespace names.
func (c *MockNamespaceMapperController) PollStartOutgoingReflection() chan string {
	panic("to implement")
}

// PollStartIncomingReflection returns the startIncomingReflection channel. It is used to receive elements to start new incoming reflection
// routines on pushed namespace names.
func (c *MockNamespaceMapperController) PollStartIncomingReflection() chan string {
	panic("to implement")
}

// PollStopOutgoingReflection returns the stopOutgoingReflectionMapper channel. It is used to receive elements to stop new outgoing reflection
// routines on pushed namespace names.
func (c *MockNamespaceMapperController) PollStopOutgoingReflection() chan string {
	panic("to implement")
}

// PollStopIncomingReflection returns the stopIncomingReflection channel. It is used to receive elements to stop new incoming reflection
// routines on pushed namespace names.
func (c *MockNamespaceMapperController) PollStopIncomingReflection() chan string {
	panic("to implement")
}

// PollStartMapper returns the startMapper channel.
func (c *MockNamespaceMapperController) PollStartMapper() chan struct{} {
	panic("to implement")
}

// PollStopMapper returns the stopMapper channel.
func (c *MockNamespaceMapperController) PollStopMapper() chan struct{} {
	panic("to implement")
}

// ReadyForRestart emits a signal to restart the OutgoingReflection.
func (c *MockNamespaceMapperController) ReadyForRestart() {
	panic("to implement")
}

// NatNamespace handle the home to foreign namespace translation. It returns an error if the mapping is not found.
func (c *MockNamespaceMapperController) NatNamespace(namespace string) (string, error) {
	return c.Mapper.NatNamespace(namespace)
}

// DeNatNamespace handle the foreign to home namespace translation. It returns an error if the mapping is not found.
func (c *MockNamespaceMapperController) DeNatNamespace(namespace string) (string, error) {
	return c.Mapper.DeNatNamespace(namespace)
}

// MappedNamespaces returns the entire namespace mapping map.
func (c *MockNamespaceMapperController) MappedNamespaces() map[string]string {
	panic("implement me")
}

// WaitForSync waits until internal caches are synchronized.
func (c *MockNamespaceMapperController) WaitForSync() {
	panic("implement me")
}
