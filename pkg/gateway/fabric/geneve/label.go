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

package geneve

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/route"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const (
	// RouteCategoryTargetValue is the value used by the routeconfiguration controller to reconcile only resources related to a gateway.
	RouteCategoryTargetValue = "gateway"

	// GeneveInterfaceNameLabelKey is the key used to store the geneve interface name in the labels.
	GeneveInterfaceNameLabelKey = "networking.liqo.io/geneve-interface-name"
)

// ForgeRouteTargetLabels returns the labels used by the routeconfiguration controller to reconcile only resources related to a single gateway.
func ForgeRouteTargetLabels(remoteID string) map[string]string {
	return map[string]string{
		route.RouteCategoryTargetKey: RouteCategoryTargetValue,
		route.RouteUniqueTargetKey:   remoteID,
	}
}

// EnsureGeneveInterfaceNamePresence ensures that the geneve interface name is present between the labels.
// If it is not present, it creates a new one.
// The string returned is the geneve interface name.
func EnsureGeneveInterfaceNamePresence(ctx context.Context, cl client.Client, internalnode *networkingv1alpha1.InternalNode) error {
	list, err := getters.ListInternalNodesByLabels(ctx, cl, labels.Everything())
	if err != nil {
		return err
	}
	name, err := FindFreeGeneveInterfaceName(list, internalnode)
	if err != nil {
		return err
	}
	internalnode.Labels[GeneveInterfaceNameLabelKey] = name
	if err := cl.Update(ctx, internalnode); err != nil {
		return err
	}
	return nil
}

// forgeGeneveInterfaceName creates a new geneve interface name starting from the InternalNode resource Name.
// The interface name can be at most 15 characters long.
func forgeGeneveInterfaceName(internalNode *networkingv1alpha1.InternalNode) string {
	if len(internalNode.Name) <= 15 {
		return internalNode.Name
	}
	return rand.String(15)
}

// GetGeneveInterfaceName returns the geneve interface name from the labels.
// If the label is not present, it returns an empty string.
func GetGeneveInterfaceName(internalNode *networkingv1alpha1.InternalNode) string {
	return internalNode.Labels[GeneveInterfaceNameLabelKey]
}

// FindFreeGeneveInterfaceName returns a free geneve interface name.
// If it cannot find a free name, it returns an error.
func FindFreeGeneveInterfaceName(list *networkingv1alpha1.InternalNodeList, internalnode *networkingv1alpha1.InternalNode) (string, error) {
	ok := false
	retry := 0
	var name string
	for !ok && retry < 20 {
		name = forgeGeneveInterfaceName(internalnode)
		ok = true
		for i := range list.Items {
			usedname := GetGeneveInterfaceName(&list.Items[i])
			if usedname == name {
				ok = false
				break
			}
		}
		retry++
	}
	if !ok {
		return "", fmt.Errorf("cannot find a free geneve interface name")
	}
	return name, nil
}
