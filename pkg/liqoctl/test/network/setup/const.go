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

package setup

const (
	// NamespaceName is the namespace where the test resources are created.
	NamespaceName = "liqo-test-network"
	// DeploymentName is the name of the deployment used for the tests.
	DeploymentName = "netshoot"
	// ControlPlaneTaintKey is the key of the taint applied to the control plane nodes.
	ControlPlaneTaintKey = "node-role.kubernetes.io/control-plane"

	// PodLabelApp is the label key used to identify the pods created by the test.
	PodLabelApp = "app"
	// PodLabelAppCluster is the label key used to identify the pods created by the test.
	PodLabelAppCluster = "app-cluster"
)
