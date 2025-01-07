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

package testutil

const (
	// EndpointIP simulate a node or a load balancer IP.
	EndpointIP = "1.0.0.1"
	// PodCIDR is the CIDR of the pod network used for testing.
	PodCIDR = "10.10.0.0/16"
	// ServiceCIDR is the CIDR of the service network used for testing.
	ServiceCIDR = "10.11.0.0/16"
	// ExternalCIDR is the external CIDR used for testing.
	ExternalCIDR = "10.12.0.0/16"
	// InternalCIDR is the internal CIDR used for testing.
	InternalCIDR = "10.13.0.0/16"
	// OverrideAPIAddress is the overrided address of the API server used for testing.
	OverrideAPIAddress = "1.0.0.2:6443"
	// ForeignAuthURL is the URL of the foreign cluster used for testing.
	ForeignAuthURL = "https://fake-auth-url:32407"
	// ForeignAPIServerURL is the URL of the foreign cluster used for testing.
	ForeignAPIServerURL = "https://fake-apiserver-url:6443"
	// ForeignProxyURL is the URL of the foreign cluster used for testing.
	ForeignProxyURL = "https://fake-proxy-url:32408"
	// VPNGatewayPort is the port of the liqo-gateway service used for testing.
	VPNGatewayPort = 32406
	// AuthenticationPort is the port of the liqo-auth service used for testing.
	AuthenticationPort = 32407
	// FakeNotReflectedLabelKey is the key of the fake not reflected label used for testing.
	FakeNotReflectedLabelKey = "not-reflected-label"
	// FakeNotReflectedAnnotKey is the key of the fake not reflected annotation used for testing.
	FakeNotReflectedAnnotKey = "not-reflected-annot"
	// FakeLiqoVersion is the version of Liqo used for testing.
	FakeLiqoVersion = "fake-version"
)

var (
	// ReservedSubnets is the list of reserved subnets used for testing.
	ReservedSubnets = []string{
		"reserved subnet 1",
		"reserved subnet 2",
		"reserved subnet 3",
		"reserved subnet 4",
	}
	// ClusterLabels is the map of labels used for testing.
	ClusterLabels = map[string]string{
		"liqo.io/testLabel": "fake label",
	}
)
