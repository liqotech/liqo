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

// Package concurrent contains the logic to manage same gateway replicas.
// They are managed using an active/passive approach.
// The gateway container try to acquire the active role using the controller manager lease.
// Then, the active gateway is labeled with the ActiveGatewayKey and ActiveGatewayValue, and the passive gateways are unlabeled.
// The gateway service target the active gateway using the ActiveGatewayKey and ActiveGatewayValue labels.
// In order to cohordinate the sidecar containers, the gateway uses a unix socket to manage the IPC, and to start the sidecars when it becomes leader.
package concurrent
