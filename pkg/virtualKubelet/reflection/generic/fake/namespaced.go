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

// NamespacedReflector implements a fake NamespacedReflector for testing purposes.
type NamespacedReflector struct {
	Opts    options.NamespacedOpts
	Handled int
	ready   bool
}

// NewNamespacedReflector returns a new fake NamespacedReflector.
func NewNamespacedReflector(opts *options.NamespacedOpts) *NamespacedReflector {
	return &NamespacedReflector{Opts: *opts}
}

// Handle increments the Handled counter.
func (r *NamespacedReflector) Handle(_ context.Context, _ string) error {
	r.Handled++
	return nil
}

// Ready returns whether the NamespacedReflector is completely initialized.
func (r *NamespacedReflector) Ready() bool { return r.ready }

// SetReady marks the NamespacedReflector as completely initialized.
func (r *NamespacedReflector) SetReady() { r.ready = true }

// List returns the list of handled namespaces.
func (r *NamespacedReflector) List() ([]interface{}, error) { return []interface{}{}, nil }
