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

package internalnetwork

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

const maxretries = 20

// FindFreeInterfaceName returns a free  interface name.
// If it cannot find a free name, it returns an error.
func FindFreeInterfaceName(ctx context.Context, cl client.Client, i interface{}) (string, error) {
	switch obj := i.(type) {
	case *networkingv1alpha1.InternalNode:
		return findFreeInterfaceNameForInternalNode(ctx, cl, obj)
	case *networkingv1alpha1.InternalFabric:
		return findFreeInterfaceNameForInternalFabric(ctx, cl, obj)
	default:
		return "", fmt.Errorf("type %T not supported", obj)
	}
}

func findFreeInterfaceNameForInternalFabric(ctx context.Context, cl client.Client,
	internalfabric *networkingv1alpha1.InternalFabric) (string, error) {
	list, err := getters.ListInternalFabricsByLabels(ctx, cl, labels.Everything())
	if err != nil {
		return "", fmt.Errorf("cannot list internal nodes: %w", err)
	}

	ok := false
	retry := 0
	var name string
	for !ok && retry < maxretries {
		name = forgeInterfaceName(internalfabric.Name)
		ok = true
		for i := range list.Items {
			if list.Items[i].Spec.Interface.Node.Name == name {
				ok = false
				break
			}
		}
		retry++
	}
	if !ok {
		return "", fmt.Errorf("cannot find a free  interface name")
	}
	return name, nil
}

func findFreeInterfaceNameForInternalNode(ctx context.Context, cl client.Client, internalfabric *networkingv1alpha1.InternalNode) (string, error) {
	list, err := getters.ListInternalNodesByLabels(ctx, cl, labels.Everything())
	if err != nil {
		return "", fmt.Errorf("cannot list internal nodes: %w", err)
	}

	ok := false
	retry := 0
	var name string
	for !ok && retry < maxretries {
		name = forgeInterfaceName(internalfabric.Name)
		ok = true
		for i := range list.Items {
			if list.Items[i].Spec.Interface.Gateway.Name == name {
				ok = false
				break
			}
		}
		retry++
	}
	if !ok {
		return "", fmt.Errorf("cannot find a free  interface name")
	}
	return name, nil
}

// forgeInterfaceName creates a new netlink interface name starting from a string.
// The interface name can be at most 15 characters long.
func forgeInterfaceName(name string) string {
	if len(name) <= 15 {
		return name
	}
	return rand.String(15)
}
