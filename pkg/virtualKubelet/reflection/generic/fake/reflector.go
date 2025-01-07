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

package fake

import (
	"context"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// Reflector implements a fake Reflector for testing purposes.
type Reflector struct {
	Started            bool
	Opts               *options.ReflectorOpts
	NamespaceStarted   map[string]*options.NamespacedOpts
	NamespaceStopped   map[string]string
	NamespaceReady     map[string]func() bool
	isLeaderRestricted bool
}

func (r *Reflector) String() string { return "fakeReflector" }

// NewReflector returns a new fake Reflector.
func NewReflector(isLeaderRestricted bool) *Reflector {
	return &Reflector{
		NamespaceStarted:   make(map[string]*options.NamespacedOpts),
		NamespaceStopped:   make(map[string]string),
		isLeaderRestricted: isLeaderRestricted,
	}
}

// Start marks the reflector as started.
func (r *Reflector) Start(_ context.Context, opts *options.ReflectorOpts) {
	r.Started = true
	r.Opts = opts
}

// StartNamespace marks the given namespace as started, and stores the given options.
func (r *Reflector) StartNamespace(opts *options.NamespacedOpts) {
	r.NamespaceStarted[opts.LocalNamespace] = opts
}

// StopNamespace marks the given namespace as stopped, and stores the remote namespace name.
func (r *Reflector) StopNamespace(local, remote string) {
	r.NamespaceStopped[local] = remote
}

// IsLeaderRestricted returns whether the reflector is restricted to the leader.
func (r *Reflector) IsLeaderRestricted() bool {
	return r.isLeaderRestricted
}

// Resync triggers a resync of the reflector.
func (r *Reflector) Resync() error {
	return nil
}
