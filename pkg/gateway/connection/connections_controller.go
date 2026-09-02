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

package connection

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/conncheck"
	"github.com/liqotech/liqo/pkg/consts"
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

	// transitions carries a GenericEvent for the Connection object every time the conncheck
	// observer detects a connected/disconnected transition, so the reconciler can flush the
	// status change immediately instead of waiting for the next periodic requeue.
	transitions chan event.GenericEvent
}

// NewConnectionsReconciler returns a new PublicKeysReconciler.
func NewConnectionsReconciler(ctx context.Context, cl client.Client,
	s *runtime.Scheme, er record.EventRecorder, options *Options) (*ConnectionsReconciler, error) {
	conncheckOpts := *options.ConnCheckOptions
	if cidr := tunnel.GetInterfaceIP(options.GwOptions.Mode, 0); cidr != "" {
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("unable to parse wireguard interface IP %q: %w", cidr, err)
		}
		conncheckOpts.PingBindIP = ip.String()
	}
	connchecker, err := conncheck.NewConnChecker(&conncheckOpts)
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
		transitions:    make(chan event.GenericEvent, 1),
	}, nil
}

// Reconcile manage PublicKey resources.
func (r *ConnectionsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	connection := &networkingv1beta1.Connection{}
	if err := r.Client.Get(ctx, req.NamespacedName, connection); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no connection %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the connection %q: %w", req.NamespacedName, err)
	}
	klog.V(4).Infof("Reconciling connection %q", req.NamespacedName)

	switch r.Options.ConnCheckOptions.PingEnabled {
	case true:
		if err := r.ensureSender(ctx, req.NamespacedName); err != nil {
			return ctrl.Result{}, err
		}

		status, err := r.ConnChecker.GetStatus(r.Options.GwOptions.RemoteClusterID)
		var (
			latency         time.Duration
			connStatusValue = networkingv1beta1.ConnectionError
		)
		if err == nil {
			latency = status.Latency
			if status.Connected {
				connStatusValue = networkingv1beta1.Connected
			}
			klog.V(6).Infof("connection %q status: connected=%v latency=%s", req.NamespacedName, status.Connected, latency)
		}

		if err := UpdateConnectionStatus(ctx, r.Client, r.Options, connection, connStatusValue, latency, time.Now()); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update the connection status: %w", err)
		}

		// Requeue periodically to flush in-memory latency to the CR.
		return ctrl.Result{RequeueAfter: r.Options.ConnCheckOptions.PingUpdateStatusInterval}, nil

	case false:
		// Ping disabled — mark the connection as connected with zero latency.
		if err := UpdateConnectionStatus(ctx, r.Client, r.Options, connection,
			networkingv1beta1.Connected, 0, time.Time{}); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update the connection status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// ensureSender adds the conncheck sender for the reconciler's remote cluster if it isn't
// already running. It is a no-op after the first successful call.
func (r *ConnectionsReconciler) ensureSender(ctx context.Context, key types.NamespacedName) error {
	clusterID := r.Options.GwOptions.RemoteClusterID
	if r.ConnChecker.HasSender(clusterID) {
		return nil
	}

	remoteIP, err := tunnel.GetRemoteInterfaceIP(r.Options.GwOptions.Mode)
	if err != nil {
		return fmt.Errorf("unable to get the remote interface IP: %w", err)
	}

	observer := onTransition(ObserveLatency(clusterID), r.enqueueTransition(key))
	if err := r.ConnChecker.AddSender(ctx, clusterID, remoteIP, observer); err != nil {
		var dupErr *conncheck.DuplicateError
		if !errors.As(err, &dupErr) {
			return fmt.Errorf("unable to add the sender: %w", err)
		}
		// Sender already added concurrently — nothing else to do.
		return nil
	}

	go r.ConnChecker.RunSender(clusterID)
	return nil
}

// onTransition wraps a conncheck.PingObserver, calling onChange whenever the connected
// state flips between two invocations, in addition to invoking the wrapped observer as usual.
func onTransition(observer conncheck.PingObserver, onChange func()) conncheck.PingObserver {
	var lastConnected atomic.Bool
	return func(connected bool, latency time.Duration) {
		observer(connected, latency)
		if lastConnected.Swap(connected) != connected {
			onChange()
		}
	}
}

func (r *ConnectionsReconciler) enqueueTransition(key types.NamespacedName) func() {
	return func() {
		select {
		case r.transitions <- event.GenericEvent{
			Object: &networkingv1beta1.Connection{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			}}:
		default:
		}
	}
}

// SetupWithManager register the ConnectionReconciler to the manager.
func (r *ConnectionsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filterByLabelsPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			consts.RemoteClusterID: r.Options.GwOptions.RemoteClusterID,
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConnection).
		For(&networkingv1beta1.Connection{}, builder.WithPredicates(filterByLabelsPredicate)).
		WatchesRawSource(source.Channel(r.transitions, &handler.EnqueueRequestForObject{})).
		Complete(r)
}
