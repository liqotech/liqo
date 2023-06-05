// Copyright 2019-2023 The Liqo Authors
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

// These annotations are  either set during the deployment of liqo using the helm
// chart or during their creation by liqo components.
// Any change to those annotations on the helm chart has also to be reflected here.

const (
	// OverrideAddressAnnotation is the annotation used to override the address of a service.
	OverrideAddressAnnotation = "liqo.io/override-address"
	// OverridePortAnnotation is the annotation used to override the port of a service.
	OverridePortAnnotation = "liqo.io/override-port"

	// APIServerSupportAnnotation is the annotation used to enable the API server support for a pod.
	APIServerSupportAnnotation = "liqo.io/api-server-support"
	// APIServerSupportAnnotationValueRemote is the value of the annotation used to enable the API server support for a pod.
	APIServerSupportAnnotationValueRemote = "remote"
	// APIServerSupportAnnotationValueDisabled is the value of the annotation used to disable the API server support for a pod.
	APIServerSupportAnnotationValueDisabled = "disabled"

	// RemoteServiceAccountNameAnnotation is the annotation used to set the name of the service account used by a pod
	// in the remote cluster. This annotation requires the API server support to be "remote" for the pod and the
	// remote service account to be created.
	RemoteServiceAccountNameAnnotation = "liqo.io/remote-service-account-name"
)
