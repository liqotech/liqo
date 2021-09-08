// Copyright 2019-2021 The Liqo Authors
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

package apiReflection

const (
	Configmaps = iota
	EndpointSlices
	Pods
	ReplicaSets
	Services
	Secrets
)

type ApiType int

var ApiNames = map[ApiType]string{
	Configmaps:     "configmaps",
	EndpointSlices: "endpointslices",
	Pods:           "pods",
	ReplicaSets:    "replicasets",
	Services:       "services",
	Secrets:        "secrets",
}

type ApiEvent struct {
	Event interface{}
	Api   ApiType
}
