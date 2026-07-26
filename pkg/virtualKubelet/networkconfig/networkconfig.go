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

// Package networkconfig provides a simple, internal network configuration representation
// used by the virtual kubelet to avoid depending on the networking API types at runtime.
package networkconfig

// CIDRPair holds a set of CIDRs and their local remappings.
type CIDRPair struct {
	Original []string
	Remapped []string
}

// RemoteCIDR holds the remote pod and external CIDRs, with their local remappings.
type RemoteCIDR struct {
	Pod      CIDRPair
	External CIDRPair
}
