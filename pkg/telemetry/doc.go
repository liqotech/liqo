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

// Package telemetry builds and sends telemetry data to the Liqo telemetry server.
// The telemetry data contains information about the cluster and the Liqo installation.
// The telemetry server aggregates the collected data and provides it to the Liqo maintainers.
// All the transmitted data is anonymous and does not contain any sensitive information.
// In particular the following data is sent:
//   - Cluster ID
//   - Liqo version
//   - Kubernetes version
//   - Node info
//     -- Kernel version
//     -- OS image
//   - Security mode
//   - Provider (e.g. GKE, EKS, AKS, ...)
//   - Peering info
//     -- RemoteClusterID
//     -- Modules
//     ---- Networking
//     ------ Enabled
//     ---- Authentication
//     ------ Enabled
//     ---- Offloading
//     ------ Enabled
//     -- Role
//     -- Latency
//     -- NodesNumber
//     -- VirtualNodesNumber
//   - Namespaces info
//     -- UID
//     -- MappingStrategy (EnforceSameName/DefaultName)
//     -- OffloadingStrategy (Local/Remote/LocalAndRemote)
//     -- HasClusterSelector (true/false)
//     -- NumOffloadedPods (map of clusterID -> number of offloaded pods)
package telemetry
