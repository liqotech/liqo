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

package connection

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
	"github.com/liqotech/liqo/pkg/gateway/tunnel"
)

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections,verbs=get;list;create;delete;update;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections/status,verbs=get;update;patch

// ConnectionsReconciler updates the PublicKey resource used to establish the Wireguard connection.
type ConnectionsReconciler struct {
	ConnChecker    *conncheck.ConnChecker
	Client         client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *Options
}

// NewConnectionsReconciler returns a new PublicKeysReconciler.
func NewConnectionsReconciler(ctx context.Context, cl client.Client,
	s *runtime.Scheme, er record.EventRecorder, options *Options) (*ConnectionsReconciler, error) {
	connchecker, err := conncheck.NewConnChecker(options.ConnCheckOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to create the connection checker: %w", err)
	}
	go connchecker.RunReceiver(ctx)
	go connchecker.RunReceiverDisconnectObserver(ctx)
	return &ConnectionsReconciler{
		ConnChecker:    connchecker,
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        options,
	}, nil
}

// Reconcile manage PublicKey resources.
func (r *ConnectionsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	connection := &networkingv1beta1.Connection{}
	if err := r.Client.Get(ctx, req.NamespacedName, connection); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no connection %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the connection %q: %w", req.NamespacedName, err)
	}
	klog.V(4).Infof("Reconciling connection %q", req.NamespacedName)

	updateConnection := ForgeUpdateConnectionCallback(ctx, r.Client, r.Options, req)

	switch r.Options.PingEnabled {
	case true:
		remoteIP, err := tunnel.GetRemoteInterfaceIP(r.Options.GwOptions.Mode)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to get the remote interface IP: %w", err)
		}

		err = r.ConnChecker.AddSender(ctx, r.Options.GwOptions.RemoteClusterID, remoteIP, updateConnection)
		if err != nil {
			switch err.(type) {
			case *conncheck.DuplicateError:
				return ctrl.Result{}, nil
			default:
				return ctrl.Result{}, fmt.Errorf("unable to add the sender: %w", err)
			}
		}

		go r.ConnChecker.RunSender(r.Options.GwOptions.RemoteClusterID)
	case false:
		if err := updateConnection(true, 0, time.Time{}); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update the connection status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the ConnectionReconciler to the manager.
func (r *ConnectionsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			string(consts.RemoteClusterID): r.Options.GwOptions.RemoteClusterID,
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConnection).
		For(&networkingv1beta1.Connection{}, builder.WithPredicates(filterByLabelsPredicate)).
		Complete(r)
}

// ForgeUpdateConnectionCallback forges the UpdateConnectionStatus function.
func ForgeUpdateConnectionCallback(ctx context.Context, cl client.Client, opts *Options, req ctrl.Request) conncheck.UpdateFunc {
	return func(connected bool, latency time.Duration, timestamp time.Time) error {
		connection := &networkingv1beta1.Connection{}
		if err := cl.Get(ctx, req.NamespacedName, connection); err != nil {
			return err
		}
		var connStatusValue networkingv1beta1.ConnectionStatusValue
		switch connected {
		case true:
			connStatusValue = networkingv1beta1.Connected
		case false:
			connStatusValue = networkingv1beta1.ConnectionError
		}
		return UpdateConnectionStatus(ctx, cl, opts, connection, connStatusValue, latency, timestamp)
	}
}
