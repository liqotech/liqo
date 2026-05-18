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

package configurationcontroller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/events"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ForgeNetworkName returns the deterministic Network resource name for a (Configuration, cidr-type, CIDR) triple.
func ForgeNetworkName(cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue, cidr networkingv1beta1.CIDR) string {
	return fmt.Sprintf("%s-%s-%s", cfg.Name, cidrType, cidrutils.EscapeForName(cidr))
}

// EnsureNetwork creates or updates an ipamv1alpha1.Network resource for one specific CIDR
// of the given Configuration and cidr-type.
func EnsureNetwork(ctx context.Context, cl client.Client, scheme *runtime.Scheme, er record.EventRecorder,
	cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue, cidr networkingv1beta1.CIDR) (*ipamv1alpha1.Network, error) {
	network := &ipamv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ForgeNetworkName(cfg, cidrType, cidr),
			Namespace: cfg.Namespace,
		},
	}

	op, err := resource.CreateOrUpdate(ctx, cl, network, func() error {
		netLabels, err := ForgeNetworkLabel(cfg, cidrType)
		if err != nil {
			return err
		}
		network.Labels = netLabels
		network.Spec.CIDR = cidr
		return ctrlutil.SetControllerReference(cfg, network, scheme)
	})
	if err != nil {
		return nil, err
	}
	if op != ctrlutil.OperationResultNone {
		events.Event(er, cfg, fmt.Sprintf("Network %s/%s %s", cfg.Namespace, network.Name, op))
	}
	return network, nil
}

// DeleteOrphanNetworks deletes or waits on Network resources owned by cfg with the given
// cidr-type label that do not match the desired steady state. It returns true when at least one
// deletion is still pending, so callers can defer creating replacement networks until the old
// ones have been fully removed and unmapped.
func DeleteOrphanNetworks(ctx context.Context, cl client.Client,
	cfg *networkingv1beta1.Configuration, cidrType LabelCIDRTypeValue, desired []networkingv1beta1.CIDR) (bool, error) {
	ls, err := ForgeNetworkLabelSelector(cfg, cidrType)
	if err != nil {
		return false, err
	}
	list, err := getters.ListNetworksByLabel(ctx, cl, cfg.Namespace, ls)
	if err != nil {
		return false, err
	}

	desiredSet := make(map[networkingv1beta1.CIDR]struct{}, len(desired))
	for i := range desired {
		desiredSet[desired[i]] = struct{}{}
	}

	pendingDeletion := false

	for i := range list.Items {
		nw := &list.Items[i]
		if !ownedByConfiguration(nw, cfg) {
			continue
		}

		expectedName := ForgeNetworkName(cfg, cidrType, nw.Spec.CIDR)
		if _, ok := desiredSet[nw.Spec.CIDR]; ok && nw.Name == expectedName {
			continue
		}

		pendingDeletion = true
		if !nw.GetDeletionTimestamp().IsZero() {
			continue
		}

		if err := cl.Delete(ctx, nw); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("unable to delete orphan Network %q: %w", nw.Name, err)
		}

		klog.Infof("Deleted orphan Network %q for configuration %q", nw.Name, client.ObjectKeyFromObject(cfg))
	}
	return pendingDeletion, nil
}

func ownedByConfiguration(obj client.Object, cfg *networkingv1beta1.Configuration) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == cfg.UID {
			return true
		}
	}
	return false
}
