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

	"k8s.io/apimachinery/pkg/types"

	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

// FallbackReflector implements a fake FallbackReflector for testing purposes.
type FallbackReflector struct {
	Opts    options.ReflectorOpts
	Handled int
	ready   bool
}

// NewFallbackReflector returns a new fake FallbackReflector.
func NewFallbackReflector(opts *options.ReflectorOpts) *FallbackReflector {
	return &FallbackReflector{Opts: *opts}
}

// Handle increments the Handled counter.
func (r *FallbackReflector) Handle(_ context.Context, _ types.NamespacedName) error {
	r.Handled++
	return nil
}

// Ready returns whether the NamespacedReflector is completely initialized.
func (r *FallbackReflector) Ready() bool { return r.ready }

// SetReady marks the NamespacedReflector as completely initialized.
func (r *FallbackReflector) SetReady() { r.ready = true }

// Keys returns a key with the namespace equal to the local namespace and the name equal to the remote one.
func (r *FallbackReflector) Keys(local, remote string) []types.NamespacedName {
	return []types.NamespacedName{{Namespace: local, Name: remote}}
}

// List returns an empty list.
func (r *FallbackReflector) List() ([]interface{}, error) {
	return nil, nil
}
