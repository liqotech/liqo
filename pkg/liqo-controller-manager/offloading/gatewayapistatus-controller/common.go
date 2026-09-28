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

package gatewayapistatusctrl

import (
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwv1 "sigs.k8s.io/gateway-api/apis/v1"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	// gatewayNameField is the field used to index the ShadowGatewayStatuses by the name of the corresponding Gateway.
	gatewayNameField = "spec.gatewayName"
	// routeNameField is the field used to index the ShadowRouteStatuses by the kind and name of the corresponding route.
	routeNameField = "spec.routeName"
)

// controllerName is the name of the controller managing the virtual GatewayClass.
var controllerName = gwv1.GatewayController(consts.GatewayControllerName)

// gatewayNameIndexer indexes the ShadowGatewayStatuses by the name of the corresponding Gateway.
func gatewayNameIndexer(obj client.Object) []string {
	shadow, ok := obj.(*offloadingv1beta1.ShadowGatewayStatus)
	if !ok {
		return nil
	}
	return []string{shadow.Spec.GatewayName}
}

// routeNameIndexer indexes the ShadowRouteStatuses by the kind and name of the corresponding route.
func routeNameIndexer(obj client.Object) []string {
	shadow, ok := obj.(*offloadingv1beta1.ShadowRouteStatus)
	if !ok {
		return nil
	}
	return []string{routeIndexKey(shadow.Spec.Kind, shadow.Spec.RouteName)}
}

func routeIndexKey(kind offloadingv1beta1.RouteKind, name string) string {
	return fmt.Sprintf("%s/%s", kind, name)
}

// parentRefKey returns a key identifying the given parent reference, normalizing the default values.
func parentRefKey(ref *gwv1.ParentReference, routeNamespace string) string {
	group, kind, namespace := gwv1.Group(gwv1.GroupName), gwv1.Kind("Gateway"), gwv1.Namespace(routeNamespace)
	if ref.Group != nil {
		group = *ref.Group
	}
	if ref.Kind != nil {
		kind = *ref.Kind
	}
	if ref.Namespace != nil {
		namespace = *ref.Namespace
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s/%d", group, kind, namespace, ref.Name, ptr.Deref(ref.SectionName, ""), ptr.Deref(ref.Port, 0))
}

// clusterCondition associates a condition with the cluster reporting it.
type clusterCondition struct {
	cluster   string
	condition *metav1.Condition
}

// aggregateConditions aggregates the conditions of the given type reported by multiple clusters: the resulting
// condition is True only if it is True in all clusters, False if it is False in any cluster, and Unknown otherwise.
func aggregateConditions(conditionType string, reported []clusterCondition, generation int64) metav1.Condition {
	sort.Slice(reported, func(i, j int) bool { return reported[i].cluster < reported[j].cluster })

	result := metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue, ObservedGeneration: generation}
	var healthy, unhealthy []string
	for _, r := range reported {
		if r.condition.Status == metav1.ConditionTrue {
			healthy = append(healthy, r.cluster)
			continue
		}

		unhealthy = append(unhealthy, fmt.Sprintf("cluster %q: %s", r.cluster, r.condition.Message))
		// The first False condition determines the reason, otherwise the first Unknown one.
		if result.Status == metav1.ConditionTrue || (result.Status == metav1.ConditionUnknown && r.condition.Status == metav1.ConditionFalse) {
			result.Status, result.Reason = r.condition.Status, r.condition.Reason
		}
	}

	if len(unhealthy) == 0 {
		result.Reason = reported[0].condition.Reason
		result.Message = fmt.Sprintf("Condition satisfied in cluster(s) %s", strings.Join(healthy, ", "))
		return result
	}
	result.Message = strings.Join(unhealthy, "; ")
	return result
}
