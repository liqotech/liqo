// Copyright 2019-2022 The Liqo Authors
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

package common

// LiqoctlShortHelp contains the short help message for root Liqoctl command.
const LiqoctlShortHelp = "liqoctl - the Liqo Command Line Interface"

// LiqoctlLongHelp contains the long help message for root Liqoctl command.
const LiqoctlLongHelp = `liqoctl is a CLI tool to install and manage Liqo-enabled clusters.

Liqo is a platform to enable dynamic and decentralized resource sharing across Kubernetes clusters. 
Liqo allows to run pods on a remote cluster seamlessly and without any modification of 
Kubernetes and the applications. 
With Liqo it is possible to extend the control plane of a Kubernetes cluster across the cluster's boundaries, 
making multi-cluster native and transparent: collapse an entire remote cluster to a virtual local node, 
by allowing workloads offloading and resource management compliant with the standard Kubernetes approach.`
