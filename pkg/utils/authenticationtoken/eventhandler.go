// Copyright 2019-2022 The Liqo Authors
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

package authenticationtoken

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// GetAuthTokenSecretPredicate returns the predicate to select the secrets containing authentication tokens
// for the remote clusters.
func GetAuthTokenSecretPredicate() predicate.Predicate {
	secretsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      discovery.ClusterIDLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
			{
				Key:      discovery.AuthTokenLabel,
				Operator: metav1.LabelSelectorOpExists,
			},
		},
	})
	if err != nil {
		klog.Fatal(err)
	}

	return secretsPredicate
}

// GetAuthTokenSecretEventHandler returns an event handler that reacts on changes over authentication token secrets
// and triggers the reconciliation of the related ForeignCluster resource (if any exists).
func GetAuthTokenSecretEventHandler(c client.Client) handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(ce event.CreateEvent, rli workqueue.RateLimitingInterface) {
			handleEvent(c, ce.Object, rli)
		},
		UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			handleEvent(c, ue.ObjectNew, rli)
		},
		DeleteFunc: func(de event.DeleteEvent, rli workqueue.RateLimitingInterface) {
			handleEvent(c, de.Object, rli)
		},
		GenericFunc: func(ge event.GenericEvent, rli workqueue.RateLimitingInterface) {},
	}
}

func handleEvent(c client.Client, object client.Object, rli workqueue.RateLimitingInterface) {
	secret, ok := object.(*v1.Secret)
	if !ok {
		klog.Errorf("object %v is not a Secret", object)
		return
	}

	ctx := context.TODO()
	if reconcileRequest := getReconcileRequestFromSecret(ctx, c, secret); reconcileRequest != nil {
		rli.Add(*reconcileRequest)
	}
}

func getReconcileRequestFromSecret(ctx context.Context, c client.Client, secret *v1.Secret) *reconcile.Request {
	labels := secret.GetLabels()
	if labels == nil {
		return nil
	}

	clusterID, ok := labels[discovery.ClusterIDLabel]
	if !ok {
		return nil
	}

	foreignCluster, err := foreignclusterutils.GetForeignClusterByID(ctx, c, clusterID)
	if err != nil {
		klog.Error(err)
		return nil
	}

	return &reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: foreignCluster.GetName(),
		},
	}
}
