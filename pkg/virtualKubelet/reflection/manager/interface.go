// Copyright 2019-2025 The Liqo Authors
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

package manager

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// Manager manages the reflection of objects between the local and the remote cluster.
type Manager interface {
	// With registers the given reflector to the manager.
	With(reflector Reflector) Manager
	// WithNamespaceHandler add the given NamespaceHandler to the manager.
	WithNamespaceHandler(handler NamespaceHandler) Manager
	// Start starts the reflection manager. It panics if executed twice.
	Start(ctx context.Context)
	// Resync triggers a resync of the reflectors.
	Resync() error

	NamespaceStartStopper
}

// NamespaceStartStopper manages the reflection at the namespace level.
type NamespaceStartStopper interface {
	// StartNamespace starts the reflection for a given namespace.
	StartNamespace(local, remote string)
	// StopNamespace stops the reflection for a given namespace.
	StopNamespace(local, remote string)
}

// Reflector implements the reflection between the local and the remote cluster.
type Reflector interface {
	// String returns the name of the reflector.
	String() string
	// Start starts the reflector.
	Start(ctx context.Context, opts *options.ReflectorOpts)
	// StartNamespace starts the reflection for the given namespace.
	StartNamespace(opts *options.NamespacedOpts)
	// StopNamespace stops the reflection for a given namespace.
	StopNamespace(local, remote string)
	// Resync triggers a resync of the reflector.
	Resync() error
}

// NamespacedReflector implements the reflection between a local and a remote namespace.
type NamespacedReflector interface {
	// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
	Handle(ctx context.Context, name string) error
	// Ready returns whether the NamespacedReflector is completely initialized.
	Ready() bool
	// List returns the list of objects to be reflected.
	List() ([]interface{}, error)
}

// FallbackReflector implements fallback reflection for "orphan" local objects not managed by namespaced reflectors.
type FallbackReflector interface {
	// Handle is responsible for reconciling the given "orphan" object.
	Handle(ctx context.Context, key types.NamespacedName) error
	// Keys returns a set of keys to be enqueued for fallback processing for the given namespace pair.
	Keys(local, remote string) []types.NamespacedName
	// Ready returns whether the FallbackReflector is completely initialized.
	Ready() bool
	// List returns the list of objects to be reflected.
	List() ([]interface{}, error)
}

// NamespaceHandler  is responsible to call StartNamespace and StopNamespace
// for a Namespace that has been marked for resources reflection.
type NamespaceHandler interface {
	// Start starts the NamespaceHandler.
	Start(context.Context, NamespaceStartStopper)
}
