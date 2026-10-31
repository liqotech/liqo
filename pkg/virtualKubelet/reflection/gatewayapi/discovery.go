// Copyright 2019-2026 The Liqo Authors
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

package gatewayapi

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	gwutils "github.com/liqotech/liqo/pkg/utils/gatewayapi"
)

// Support describes how the reflection of a given Gateway API resource shall be performed.
type Support string

const (
	// SupportNone means that the resource is not served by the local cluster, hence there is nothing to reflect.
	SupportNone Support = "None"
	// SupportDegraded means that the resource is served by the local cluster only: local objects are not reflected,
	// but warning events are generated to notify that the reflection towards the remote cluster is not possible.
	SupportDegraded Support = "Degraded"
	// SupportFull means that the resource is served by both clusters, hence it can be reflected.
	SupportFull Support = "Full"
)

// SupportFor returns the type of reflection supported for the given resource, depending on the availability in both clusters.
func SupportFor(gvr schema.GroupVersionResource, local, remote gwutils.Availability) Support {
	switch {
	case !local.Has(gvr):
		return SupportNone
	case !remote.Has(gvr):
		return SupportDegraded
	default:
		return SupportFull
	}
}
