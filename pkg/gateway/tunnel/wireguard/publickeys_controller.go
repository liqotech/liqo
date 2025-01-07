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

package wireguard

import (
	"context"
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=publickeies,verbs=get;list;create;delete;update;watch

// PublicKeysReconciler updates the PublicKey resource used to establish the Wireguard connection.
type PublicKeysReconciler struct {
	Wgcl           *wgctrl.Client
	Client         client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *Options
}

// NewPublicKeysReconciler returns a new PublicKeysReconciler.
func NewPublicKeysReconciler(cl client.Client, s *runtime.Scheme, er record.EventRecorder, options *Options) (*PublicKeysReconciler, error) {
	wgcl, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("unable to create wireguard client: %w", err)
	}
	return &PublicKeysReconciler{
		Wgcl:           wgcl,
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        options,
	}, nil
}

// Reconcile manage PublicKey resources.
func (r *PublicKeysReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	publicKey := &networkingv1beta1.PublicKey{}
	if err := r.Client.Get(ctx, req.NamespacedName, publicKey); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no publicKey %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the publicKey %q: %w", req.NamespacedName, err)
	}

	if r.Options.GwOptions.Mode == gateway.ModeClient && r.Options.EndpointIP == nil {
		// We don't need to retry because the DNS resolution routine will wakeup this controller.
		klog.Warning("EndpointIP is not set yet. Maybe the DNS resolution is still in progress")
		return ctrl.Result{}, nil
	}

	if err := configureDevice(r.Wgcl, r.Options, wgtypes.Key(publicKey.Spec.PublicKey)); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, EnsureConnection(ctx, r.Client, r.Scheme, r.Options)
}

// SetupWithManager register the ConfigurationReconciler to the manager.
func (r *PublicKeysReconciler) SetupWithManager(mgr ctrl.Manager, src <-chan event.GenericEvent) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlPublicKey).
		For(&networkingv1beta1.PublicKey{}, r.Predicates()).
		WatchesRawSource(NewDNSSource(src, NewDNSEventHandler(r.Client, r.Options))).
		Complete(r)
}

// Predicates returns the predicates required for the PublicKey controller.
func (r *PublicKeysReconciler) Predicates() builder.Predicates {
	return builder.WithPredicates(
		predicate.NewPredicateFuncs(func(object client.Object) bool {
			id, ok := object.GetLabels()[string(consts.RemoteClusterID)]
			if !ok {
				return false
			}
			return id == r.Options.GwOptions.RemoteClusterID
		}))
}
