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

package resources

// ResourceReflected represents a resource that can be reflected.
type ResourceReflected string

// List of all resources that can be reflected.
const (
	Pod                   ResourceReflected = "pod"
	Service               ResourceReflected = "service"
	EndpointSlice         ResourceReflected = "endpointslice"
	Ingress               ResourceReflected = "ingress"
	ConfigMap             ResourceReflected = "configmap"
	Secret                ResourceReflected = "secret"
	ServiceAccount        ResourceReflected = "serviceaccount"
	PersistentVolumeClaim ResourceReflected = "persistentvolumeclaim"
	Event                 ResourceReflected = "event"
)

// Reflectors is the list of all resources that can be reflected.
var Reflectors = []ResourceReflected{Pod, Service, EndpointSlice, Ingress, ConfigMap, Secret, ServiceAccount, PersistentVolumeClaim, Event}

// ReflectorsCustomizableType is the list of resources for which the reflection type can be customized.
var ReflectorsCustomizableType = []ResourceReflected{Service, Ingress, ConfigMap, Secret, Event}
