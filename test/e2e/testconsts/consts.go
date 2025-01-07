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

package testconsts

// Environment variable.
const (
	NamespaceEnvVar        = "NAMESPACE"
	ClusterNumberVarKey    = "CLUSTER_NUMBER"
	KubeconfigDirVarName   = "KUBECONFIGDIR"
	LiqoctlPathEnvVar      = "LIQOCTL"
	InfrastructureEnvVar   = "INFRA"
	CniEnvVar              = "CNI"
	OverlappingCIDRsEnvVar = "POD_CIDR_OVERLAPPING"
)

// LiqoTestNamespaceLabels is a set of labels that has to be attached to test namespaces to simplify garbage collection.
var LiqoTestNamespaceLabels = map[string]string{
	LiqoTestingLabelKey: LiqoTestingLabelValue,
}

const (
	// Keys for cluster labels.

	// ProviderKey indicates the cluster provider.
	ProviderKey = "provider"
	// RegionKey indicates the cluster region.
	RegionKey = "region"

	// Values for cluster labels.

	// ProviderAzure -> provider=Azure.
	ProviderAzure = "Azure"
	// ProviderAWS -> provider=AWS.
	ProviderAWS = "AWS"
	// ProviderGKE -> provider=GKE.
	ProviderGKE = "GKE"
	// ProviderK3s -> provider=K3s.
	ProviderK3s = "k3s"
	// RegionA -> region=A.
	RegionA = "A"
	// RegionB -> region=B.
	RegionB = "B"
	// RegionC -> region=C.
	RegionC = "C"
	// RegionD -> region=D.
	RegionD = "D"

	// LiqoTestingLabelKey is a label that has to be attached to test namespaces to simplify garbage collection.
	LiqoTestingLabelKey = "liqo.io/testing-namespace"
	// LiqoTestingLabelValue is the value of the LiqoTestingLabelKey.
	LiqoTestingLabelValue = "true"
)
