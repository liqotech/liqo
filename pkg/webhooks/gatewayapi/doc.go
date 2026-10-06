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

// Package gatewayapi contains the webhooks enforcing, in the provider cluster, the Gateway API resources reflected from
// the consumer clusters: the placeholder of the shared Gateway is replaced with the actual shared Gateway, and the reflected
// resources are prevented from referring to resources not belonging to the consumer cluster (e.g., Gateways not shared).
package gatewayapi
