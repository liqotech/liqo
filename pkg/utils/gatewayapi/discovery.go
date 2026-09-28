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
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"
)

var (
	groupVersion = schema.GroupVersion{Group: gwv1.GroupVersion.Group, Version: gwv1.GroupVersion.Version}

	// GatewayClassesGVR is the GroupVersionResource of the GatewayClass resource.
	GatewayClassesGVR = groupVersion.WithResource("gatewayclasses")
	// GatewaysGVR is the GroupVersionResource of the Gateway resource.
	GatewaysGVR = groupVersion.WithResource("gateways")
	// HTTPRoutesGVR is the GroupVersionResource of the HTTPRoute resource.
	HTTPRoutesGVR = groupVersion.WithResource("httproutes")
	// GRPCRoutesGVR is the GroupVersionResource of the GRPCRoute resource.
	GRPCRoutesGVR = groupVersion.WithResource("grpcroutes")
	// ReferenceGrantsGVR is the GroupVersionResource of the ReferenceGrant resource.
	ReferenceGrantsGVR = groupVersion.WithResource("referencegrants")

	// Resources lists the Gateway API resources handled by Liqo.
	Resources = []schema.GroupVersionResource{GatewayClassesGVR, GatewaysGVR, HTTPRoutesGVR, GRPCRoutesGVR, ReferenceGrantsGVR}
)

// Availability describes which of the Gateway API resources handled by Liqo are served by a cluster.
type Availability map[schema.GroupVersionResource]bool

// Has returns whether the given resource is served by the cluster.
func (a Availability) Has(gvr schema.GroupVersionResource) bool {
	return a[gvr]
}

// Detect returns which of the Gateway API resources handled by Liqo are served by the cluster,
// to avoid starting informers (or controllers) for missing CRDs, which would never sync.
func Detect(client discovery.DiscoveryInterface) (Availability, error) {
	availability := make(Availability, len(Resources))

	served := make(map[string]map[string]bool)
	for _, gvr := range Resources {
		gv := gvr.GroupVersion().String()
		if _, found := served[gv]; !found {
			list, err := client.ServerResourcesForGroupVersion(gv)
			switch {
			case kerrors.IsNotFound(err):
				// The group version is not served, hence none of its resources is available.
				served[gv] = map[string]bool{}
			case err != nil:
				return nil, fmt.Errorf("failed to discover the resources of group version %q: %w", gv, err)
			default:
				served[gv] = make(map[string]bool, len(list.APIResources))
				for i := range list.APIResources {
					served[gv][list.APIResources[i].Name] = true
				}
			}
		}

		availability[gvr] = served[gv][gvr.Resource]
	}

	return availability, nil
}
