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

package namespacesmapping

import (
	"context"

	"k8s.io/client-go/rest"
)

// MapperController is the interface used to implement the MapperController. It encapsulates methods to control:
// - Outgoing Reflection
// - Incoming Reflection (NamespaceMirroringController)
// - Namespace Mapping.
type MapperController interface {
	NamespaceReflectionController
	NamespaceMirroringController
	NamespaceNatter

	PollStartMapper() chan struct{}
	PollStopMapper() chan struct{}
	ReadyForRestart()
}

// NamespaceReflectionController defines the interface outgoing reflection, defining methods to extract items to start/stop
// outgoing reflections.
type NamespaceReflectionController interface {
	PollStartOutgoingReflection() chan string
	PollStopOutgoingReflection() chan string
}

// NamespaceMirroringController defines the interface incoming reflection (mirroring), defining methods to extract items to start/stop
// incoming reflections.
type NamespaceMirroringController interface {
	PollStartIncomingReflection() chan string
	PollStopIncomingReflection() chan string
}

// NamespaceNatter defines the interface for namespace translation, defining methods to obtain home/foreign namespace from the corresponding one.
type NamespaceNatter interface {
	NatNamespace(namespace string) (string, error)
	DeNatNamespace(namespace string) (string, error)
	MappedNamespaces() map[string]string
}

// NamespaceMapperController handles namespace translation and reflection by implementing NamespaceNatter,NamespaceMirroringController
// and NamespaceReflectionController interface.
type NamespaceMapperController struct {
	mapper *NamespaceMapper
}

// NewNamespaceMapperController builds and returns a new NewNamespaceMapperController.
func NewNamespaceMapperController(ctx context.Context, config *rest.Config,
	homeClusterID, foreignClusterID, namespace string) (*NamespaceMapperController, error) {
	controller := &NamespaceMapperController{
		mapper: &NamespaceMapper{
			homeClusterID:           homeClusterID,
			foreignClusterID:        foreignClusterID,
			namespace:               namespace,
			startOutgoingReflection: make(chan string, 100),
			startIncomingReflection: make(chan string, 100),
			stopIncomingReflection:  make(chan string, 100),
			stopOutgoingReflection:  make(chan string, 100),
			startMapper:             make(chan struct{}, 100),
			stopMapper:              make(chan struct{}, 100),
			restartReady:            make(chan struct{}, 100),
		},
	}

	if err := controller.mapper.init(ctx, config); err != nil {
		return nil, err
	}

	return controller, nil
}

// PollStartOutgoingReflection returns the startOutgoingReflection channel. It is used to receive elements to start new outgoing reflection
// routines on pushed namespace names.
func (c *NamespaceMapperController) PollStartOutgoingReflection() chan string {
	return c.mapper.startOutgoingReflection
}

// PollStartIncomingReflection returns the startIncomingReflection channel. It is used to receive elements to start new incoming reflection
// routines on pushed namespace names.
func (c *NamespaceMapperController) PollStartIncomingReflection() chan string {
	return c.mapper.startIncomingReflection
}

// PollStopOutgoingReflection returns the stopOutgoingReflectionMapper channel. It is used to receive elements to stop new outgoing reflection
// routines on pushed namespace names.
func (c *NamespaceMapperController) PollStopOutgoingReflection() chan string {
	return c.mapper.stopOutgoingReflection
}

// PollStopIncomingReflection returns the stopIncomingReflection channel. It is used to receive elements to stop new incoming reflection
// routines on pushed namespace names.
func (c *NamespaceMapperController) PollStopIncomingReflection() chan string {
	return c.mapper.stopIncomingReflection
}

// PollStartMapper returns the startMapper channel.
func (c *NamespaceMapperController) PollStartMapper() chan struct{} {
	return c.mapper.startMapper
}

// PollStopMapper returns the stopMapper channel.
func (c *NamespaceMapperController) PollStopMapper() chan struct{} {
	return c.mapper.stopMapper
}

// ReadyForRestart emits a signal to restart the OutgoingReflection.
func (c *NamespaceMapperController) ReadyForRestart() {
	c.mapper.restartReady <- struct{}{}
}

// NatNamespace handles the home to foreign namespace translation. It returns an error if the mapping is not found.
func (c *NamespaceMapperController) NatNamespace(namespace string) (string, error) {
	return c.mapper.HomeToForeignNamespace(namespace)
}

// DeNatNamespace handles the foreign to home namespace translation. It returns an error if the mapping is not found.
func (c *NamespaceMapperController) DeNatNamespace(namespace string) (string, error) {
	return c.mapper.ForeignToLocalNamespace(namespace)
}

// WaitForSync waits until internal caches are synchronized.
func (c *NamespaceMapperController) WaitForSync() {
	c.mapper.WaitNamespaceNattingTableSync(context.Background())
}

// MappedNamespaces returns the entire namespace mapping map.
func (c *NamespaceMapperController) MappedNamespaces() map[string]string {
	return c.mapper.MappedNamespaces()
}
