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

package routeconfiguration

import (
	"context"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

func checkUniqueTableName(ctx context.Context, cl client.Client, routeconfiguration *networkingv1beta1.RouteConfiguration) error {
	routeConfigurationList := &networkingv1beta1.RouteConfigurationList{}
	if err := cl.List(ctx, routeConfigurationList); err != nil {
		return err
	}
	for i := range routeConfigurationList.Items {
		if routeConfigurationList.Items[i].UID == routeconfiguration.UID {
			continue
		}
		if routeConfigurationList.Items[i].Spec.Table.Name == routeconfiguration.Spec.Table.Name &&
			maps.Equal(routeConfigurationList.Items[i].GetLabels(), routeconfiguration.GetLabels()) {
			return fmt.Errorf("table name %s already used", routeconfiguration.Spec.Table.Name)
		}
	}
	return nil
}

func checkImmutableTableName(newroutecfg, oldroutecfg *networkingv1beta1.RouteConfiguration) error {
	if newroutecfg.Spec.Table.Name != oldroutecfg.Spec.Table.Name {
		return fmt.Errorf("table name is immutable and cannot be changed: %s -> %s",
			oldroutecfg.Spec.Table.Name, newroutecfg.Spec.Table.Name)
	}
	return nil
}
