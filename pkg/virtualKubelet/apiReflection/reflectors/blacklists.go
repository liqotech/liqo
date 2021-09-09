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

package reflectors

import (
	apimgmt "github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection"
)

type blackListType map[string]struct{}

// the blacklist is a map containing the objects that should not be managed by the reflectors.
// the blacklist is generally checked in the `isAllowed` method of the reflectors
// TODO: in a future version we could/should move to a dynamic blaklisting package with contexts.
var Blacklist = map[apimgmt.ApiType]blackListType{
	apimgmt.EndpointSlices: {
		"default/kubernetes": struct{}{},
	},
	apimgmt.Pods: {},
	apimgmt.Services: {
		"default/kubernetes": struct{}{},
	},
}
