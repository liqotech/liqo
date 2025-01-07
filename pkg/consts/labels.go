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

package consts

// These labels are  either set during the deployment of liqo using the helm chart
// or during their creation by liqo components.
// Any change to those labels on the helm chart has also to be reflected here.
// ServiceLabelKey key of the label added to the Gateway service. Used to get the
// service by label.

const (
	// K8sAppNameKey = key of the label used to denote a deployed application name.
	K8sAppNameKey = "app.kubernetes.io/name"
	// K8sAppInstanceKey = key of the label used to denote a deployed application instance.
	K8sAppInstanceKey = "app.kubernetes.io/instance"
	// K8sAppManagedByKey = key of the label used to denote which app is managing the resource.
	K8sAppManagedByKey = "app.kubernetes.io/managed-by"
	// K8sAppComponentKey = key of the label used to denote a deployed application component.
	K8sAppComponentKey = "app.kubernetes.io/component"
	// K8sAppPartOfKey = key of the label used to denote the application a resource is part of.
	K8sAppPartOfKey = "app.kubernetes.io/part-of"

	// ControllerManagerAppName label value that denotes the name of the liqo-controller-manager deployment.
	ControllerManagerAppName = "controller-manager"

	// APIServerProxyAppName label value that denotes the name of the liqo-api-server-proxy deployment.
	APIServerProxyAppName = "proxy"

	// OffloadingComponentKey is the label assigned to the Liqo components related to offloading.
	OffloadingComponentKey = "offloading.liqo.io/component"

	// VirtualKubeletComponentValue is the value to use with the OffloadingComponentKey to label the Virtual Kubelet component.
	VirtualKubeletComponentValue = "virtual-kubelet"

	// NetworkingComponentKey is the label assigned to the Liqo components related to networking.
	NetworkingComponentKey = "networking.liqo.io/component"

	// WebhookResourceLabelKey is the constant representing
	// the key of the label assigned to all Webhook resources.
	WebhookResourceLabelKey = "liqo.io/webhook"
	// WebhookResourceLabelValue is the constant representing
	// the value of the label assigned to all Webhook resources.
	WebhookResourceLabelValue = "true"
)
