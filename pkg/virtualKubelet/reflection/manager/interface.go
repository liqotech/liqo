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

package manager

import (
	"context"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// Manager manages the reflection of objects between the local and the remote cluster.
type Manager interface {
	// With registers the given reflector to the manager.
	With(reflector Reflector) Manager
	// Start starts the reflection manager. It panics if executed twice.
	Start(ctx context.Context)

	// StartNamespace starts the reflection for a given namespace.
	StartNamespace(local, remote string)
	// StopNamespace stops the reflection for a given namespace.
	StopNamespace(local, remote string)
}

// Reflector implements the reflection between the local and the remote cluster.
type Reflector interface {
	// Start starts the reflector.
	Start(ctx context.Context)
	// StartNamespace starts the reflection for the given namespace.
	StartNamespace(opts *options.ReflectorOpts)
	// StopNamespace stops the reflection for a given namespace.
	StopNamespace(localNamespace, remoteNamespace string)
	// SetNamespaceReady marks the given namespace as ready for resource reflection.
	SetNamespaceReady(namespace string)
}

// NamespacedReflector implements the reflection between a local and a remote namespace.
type NamespacedReflector interface {
	// Handle is responsible for reconciling the given object and ensuring it is correctly reflected.
	Handle(ctx context.Context, name string) error

	// Ready returns whether the NamespacedReflector is completely initialized.
	Ready() bool
	// SetReady marks the NamespacedReflector as completely initialized.
	SetReady()
}
