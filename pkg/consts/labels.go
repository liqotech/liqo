// Copyright 2019-2024 The Liqo Authors
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

package consts

// These labels are  either set during the deployment of liqo using the helm chart
// or during their creation by liqo components.
// Any change to those labels on the helm chart has also to be reflected here.
// ServiceLabelKey key of the label added to the Gateway service. Used to get the
// service by label.

const (
	// K8sAppNameKey = key of the label used to denote a deployed application name.
	K8sAppNameKey = "app.kubernetes.io/name"
	// K8sAppComponentKey = key of the label used to denote a deployed application component.
	K8sAppComponentKey = "app.kubernetes.io/component"

	// AuthAppName label value that denotes the name of the liqo-auth deployment.
	AuthAppName = "auth"

	// ControllerManagerAppName label value that denotes the name of the liqo-controller-manager deployment.
	ControllerManagerAppName = "controller-manager"

	// APIServerProxyAppName label value that denotes the name of the liqo-api-server-proxy deployment.
	APIServerProxyAppName = "proxy"

	// IpamStorageResourceLabelKey is the constant representing
	// the key of the label assigned to all IpamStorage resources.
	IpamStorageResourceLabelKey = "ipam.liqo.io/ipamstorage"
	// IpamStorageResourceLabelValue is the constant representing
	// the value of the label assigned to all IpamStorage resources.
	IpamStorageResourceLabelValue = "true"
)
