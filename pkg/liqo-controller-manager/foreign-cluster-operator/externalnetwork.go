// Copyright 2019-2024 The Liqo Authors
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

package foreignclusteroperator

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

func (r *ForeignClusterReconciler) ensureExternalNetwork(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	klog.Infof("[%s] ensuring ExternalNetwork existence", fc.Spec.ClusterIdentity.ClusterID)

	if exists, err := r.checkForOtherGateways(ctx, fc); err != nil {
		klog.Error(err)
		return err
	} else if exists {
		klog.Infof("[%s] ExternalNetwork already exists and is not managed by ExternalNetwork", fc.Spec.ClusterIdentity.ClusterID)
		return nil
	}

	localNamespace := fc.Status.TenantNamespace.Local

	conf, err := configuration.ForgeConfigurationForRemoteCluster(ctx, r.Client, localNamespace, r.LiqoNamespace)
	if err != nil {
		klog.Error(err)
		return err
	}

	extNet := &networkingv1alpha1.ExternalNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getResourceRequestNameFor(r.HomeCluster),
			Namespace: localNamespace,
		},
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, extNet, func() error {
		if extNet.Labels == nil {
			extNet.Labels = map[string]string{}
		}
		extNet.Labels[consts.ReplicationDestinationLabel] = fc.Spec.ClusterIdentity.ClusterID
		extNet.Labels[consts.ReplicationRequestedLabel] = consts.ReplicationRequestedLabelValue

		extNet.Spec.Configuration = conf.Spec.DeepCopy()
		extNet.Spec.ClusterIdentity = r.HomeCluster.DeepCopy()

		return controllerutil.SetControllerReference(fc, extNet, r.Scheme)
	}); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (r *ForeignClusterReconciler) deleteExternalNetwork(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) error {
	klog.Infof("[%s] deleting ExternalNetwork", fc.Spec.ClusterIdentity.ClusterID)

	localNamespace := fc.Status.TenantNamespace.Local

	extNet := &networkingv1alpha1.ExternalNetwork{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getResourceRequestNameFor(r.HomeCluster),
			Namespace: localNamespace,
		},
	}

	if err := client.IgnoreNotFound(r.Client.Delete(ctx, extNet)); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (r *ForeignClusterReconciler) checkForOtherGateways(ctx context.Context, fc *discoveryv1alpha1.ForeignCluster) (exists bool, err error) {
	gwServer, err := getters.GetGatewayServerByClusterID(ctx, r.Client, &fc.Spec.ClusterIdentity)
	switch {
	case apierrors.IsNotFound(err):
		// no GatewayServer found
	case err != nil:
		return false, err
	default:
		if gwServer.OwnerReferences == nil || len(gwServer.OwnerReferences) == 0 {
			return true, nil
		}
		for i := range gwServer.OwnerReferences {
			ownerRef := gwServer.OwnerReferences[i]
			if ownerRef.Kind == networkingv1alpha1.ExternalNetworkKind {
				// the GatewayServer is managed by the ExternalNetwork, so we can enforce it by ignoring its existence
				return false, nil
			}
		}
		return true, nil
	}

	wgClient, err := getters.GetGatewayClientByClusterID(ctx, r.Client, &fc.Spec.ClusterIdentity)
	switch {
	case apierrors.IsNotFound(err):
		// no GatewayClient found
	case err != nil:
		return false, err
	default:
		if wgClient.OwnerReferences == nil || len(wgClient.OwnerReferences) == 0 {
			return true, nil
		}
		for i := range wgClient.OwnerReferences {
			ownerRef := wgClient.OwnerReferences[i]
			if ownerRef.Kind == networkingv1alpha1.ExternalNetworkKind {
				// the GatewayClient is managed by the ExternalNetwork, so we can enforce it by ignoring its existence
				return false, nil
			}
		}
		return true, nil
	}

	return false, nil
}
