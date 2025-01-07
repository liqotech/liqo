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

package internalfabriccontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/id"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

func geneveTunnelName(internalFabric *networkingv1beta1.InternalFabric, internalNode *networkingv1beta1.InternalNode) string {
	return fmt.Sprintf("%s-%s", internalFabric.Name, internalNode.Name)
}

func mutateGeneveTunnel(ctx context.Context, cl client.Client, tunnel *networkingv1beta1.GeneveTunnel,
	internalFabric *networkingv1beta1.InternalFabric, internalNode *networkingv1beta1.InternalNode) error {
	if tunnel.Labels == nil {
		tunnel.Labels = make(map[string]string)
	}

	tunnel.Labels[consts.InternalFabricName] = internalFabric.Name
	tunnel.Labels[consts.InternalNodeName] = internalNode.Name

	tunnelID, err := id.GetGeneveTunnelManager(ctx, cl).Allocate(client.ObjectKeyFromObject(tunnel).String())
	if err != nil {
		return err
	}

	tunnel.Spec.ID = tunnelID

	tunnel.Spec.InternalNodeRef = &corev1.ObjectReference{
		Name: internalNode.Name,
	}
	tunnel.Spec.InternalFabricRef = &corev1.ObjectReference{
		Name:      internalFabric.Name,
		Namespace: internalFabric.Namespace,
	}
	return nil
}

func cleanupGeneveTunnels(ctx context.Context, cl client.Client,
	internalFabric *networkingv1beta1.InternalFabric, internalNodeList *networkingv1beta1.InternalNodeList) error {
	var tunnelList networkingv1beta1.GeneveTunnelList
	if err := cl.List(ctx, &tunnelList, client.InNamespace(internalFabric.Namespace), client.MatchingLabels{
		consts.InternalFabricName: internalFabric.Name,
	}); err != nil {
		return err
	}

	var nodes = make(map[string]any)
	for i := range internalNodeList.Items {
		node := &internalNodeList.Items[i]
		nodes[node.Name] = nil
	}

	for i := range tunnelList.Items {
		tunnel := &tunnelList.Items[i]
		if _, ok := nodes[tunnel.Labels[consts.InternalNodeName]]; !ok {
			id.GetGeneveTunnelManager(ctx, cl).Release(client.ObjectKeyFromObject(tunnel).String())
			if err := client.IgnoreNotFound(cl.Delete(ctx, tunnel)); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureGeneveTunnels(ctx context.Context, cl client.Client, s *runtime.Scheme,
	internalFabric *networkingv1beta1.InternalFabric, internalNodeList *networkingv1beta1.InternalNodeList) error {
	for i := range internalNodeList.Items {
		node := &internalNodeList.Items[i]

		name := geneveTunnelName(internalFabric, node)
		tunnel := &networkingv1beta1.GeneveTunnel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: internalFabric.Namespace,
			},
		}

		if _, err := resource.CreateOrUpdate(ctx, cl, tunnel, func() error {
			if err := mutateGeneveTunnel(ctx, cl, tunnel, internalFabric, node); err != nil {
				klog.Errorf("Unable to mutate GeneveTunnel %q: %s", client.ObjectKeyFromObject(tunnel).String(), err)
				return err
			}
			return controllerutil.SetControllerReference(internalFabric, tunnel, s)
		}); err != nil {
			return err
		}
	}

	if len(internalNodeList.Items) > 0 {
		updated := controllerutil.AddFinalizer(internalFabric, consts.InternalFabricGeneveTunnelFinalizer)
		if updated {
			if err := cl.Update(ctx, internalFabric); err != nil {
				klog.Errorf("Unable to update InternalFabric %q: %s", client.ObjectKeyFromObject(internalFabric).String(), err)
				return err
			}
			return nil
		}
	}

	return nil
}

func deleteGeneveTunnels(ctx context.Context, cl client.Client,
	internalFabric *networkingv1beta1.InternalFabric) error {
	// delete geneve tunnels
	var geneveTunnelList networkingv1beta1.GeneveTunnelList
	if err := cl.List(ctx, &geneveTunnelList, client.InNamespace(internalFabric.Namespace), client.MatchingLabels{
		consts.InternalFabricName: internalFabric.Name,
	}); err != nil {
		klog.Errorf("Unable to list GeneveTunnels: %s", err)
		return err
	}

	for i := range geneveTunnelList.Items {
		tunnel := &geneveTunnelList.Items[i]
		id.GetGeneveTunnelManager(ctx, cl).Release(client.ObjectKeyFromObject(tunnel).String())
		if err := client.IgnoreNotFound(cl.Delete(ctx, tunnel)); err != nil {
			klog.Errorf("Unable to delete GeneveTunnel %q: %s", client.ObjectKeyFromObject(tunnel), err)
			return err
		}
	}

	updated := controllerutil.RemoveFinalizer(internalFabric, consts.InternalFabricGeneveTunnelFinalizer)
	if updated {
		if err := cl.Update(ctx, internalFabric); err != nil {
			klog.Errorf("Unable to update InternalFabric %q: %s", client.ObjectKeyFromObject(internalFabric).String(), err)
			return err
		}
	}

	return nil
}
