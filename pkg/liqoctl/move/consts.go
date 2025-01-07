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

package move

const (
	liqoStorageNamespace = "liqo-storage"
	resticRegistry       = "restic-registry"
	resticPort           = 8000

	// DefaultResticServerImage is the default image used for the restic server.
	DefaultResticServerImage = "restic/rest-server:0.11.0"
	// DefaultResticImage is the default image used for the restic client.
	DefaultResticImage = "restic/restic:0.14.0"
)
