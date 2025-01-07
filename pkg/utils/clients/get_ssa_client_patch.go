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

// Package clients contains utility methods to create and manage clients with custom features.
package clients

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ssaPatch uses server-side apply to patch the object.
type ssaPatch struct {
	patch any
}

// Patch returns a client.Patch object to perform a server side apply operation, associated with the given configuration.
// The argument must be a pointer to an *ApplyConfiguration object (e.g., PodApplyConfiguration).
func Patch(patch any) client.Patch {
	return ssaPatch{patch: patch}
}

// Type implements the client.Patch interface.
func (p ssaPatch) Type() types.PatchType {
	return types.ApplyPatchType
}

// Data implements the client.Patch interface.
func (p ssaPatch) Data(_ client.Object) ([]byte, error) {
	return json.Marshal(p.patch)
}
