// Copyright 2019-2026 The Liqo Authors
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

// ClusterReflector implements a fake cluster-scoped reflector for testing purposes.
type ClusterReflector struct {
	Opts    options.ReflectorOpts
	Handled int
	ready   bool
}

// NewClusterReflector returns a new fake cluster-scoped reflector.
func NewClusterReflector(opts *options.ReflectorOpts) *ClusterReflector {
	return &ClusterReflector{Opts: *opts}
}

// Handle increments the Handled counter.
func (r *ClusterReflector) Handle(_ context.Context, _ string) error {
	r.Handled++
	return nil
}

// Ready returns whether the reflector is completely initialized.
func (r *ClusterReflector) Ready() bool { return r.ready }

// SetReady marks the reflector as completely initialized.
func (r *ClusterReflector) SetReady() { r.ready = true }

// List returns an empty list.
func (r *ClusterReflector) List() ([]any, error) { return nil, nil }
