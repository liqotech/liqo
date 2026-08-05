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

package fabric

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/conncheck"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/network/geneve"
	timeutils "github.com/liqotech/liqo/pkg/utils/time"
)

// GeneveTunnelReconciler manages geneve tunnels for the fabric.
type GeneveTunnelReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder events.EventRecorder
	Options        *Options

	connChecker     atomic.Pointer[conncheck.ConnChecker]
	connCheckerErr  error
	connCheckerOnce sync.Once

	// transitions carries a GenericEvent for the Connection object every time the conncheck
	// observer detects a connected/disconnected transition, so the reconciler can flush the
	// status change immediately instead of waiting for the next periodic requeue.
	transitions chan event.GenericEvent
}

// NewGeneveTunnelReconciler returns a new GeneveTunnelReconciler.
func NewGeneveTunnelReconciler(cl client.Client, s *runtime.Scheme,
	er events.EventRecorder, opts *Options) (*GeneveTunnelReconciler, error) {
	return &GeneveTunnelReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        opts,
		transitions:    make(chan event.GenericEvent, 10),
	}, nil
}

// initConnCheckerReceiver creates and starts the ConnChecker receiver bound to the node inner geneve IP.
// It is guaranteed to run at most once.
func (r *GeneveTunnelReconciler) initConnCheckerReceiver(ctx context.Context, bindIP string) error {
	r.connCheckerOnce.Do(func() {
		opts := r.Options.ConnCheckOptions
		opts.PingBindIP = bindIP
		cc, err := conncheck.NewConnChecker(opts)
		if err != nil {
			r.connCheckerErr = fmt.Errorf("creating geneve conncheck receiver: %w", err)
			return
		}
		go cc.RunReceiver(ctx)
		go cc.RunReceiverDisconnectObserver(ctx)
		r.connChecker.Store(cc)
		klog.Infof("geneve conncheck receiver started, bound to %s:%d", bindIP, opts.PingPort)
	})
	return r.connCheckerErr
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch

// Reconcile manages GeneveTunnels.
func (r *GeneveTunnelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	gt := &networkingv1beta1.GeneveTunnel{}
	if err := r.Get(ctx, req.NamespacedName, gt); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("GeneveTunnel %s not found", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting genevetunnel %q: %w", req.NamespacedName, err)
	}

	if !gt.DeletionTimestamp.IsZero() {
		r.stopSender(gt.Name)
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("Reconciling genevetunnel %s", req.String())

	if gt.Spec.InternalNodeRef == nil || gt.Spec.InternalFabricRef == nil {
		klog.V(4).Infof("Skipping genevetunnel %s: missing internalNodeRef or internalFabricRef", req.String())
		return ctrl.Result{}, nil
	}

	// This should never happen due to the label selector predicate, but guard against it for extra caution.
	if gt.Spec.InternalNodeRef.Name != r.Options.NodeName {
		klog.V(4).Infof("Skipping genevetunnel %s: not targeting node %s", req.String(), r.Options.NodeName)
		return ctrl.Result{}, nil
	}

	var internalfabric networkingv1beta1.InternalFabric
	err := r.Get(ctx, types.NamespacedName{
		Name:      gt.Spec.InternalFabricRef.Name,
		Namespace: gt.Spec.InternalFabricRef.Namespace,
	}, &internalfabric)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting internalfabric %s/%s: %w",
			gt.Spec.InternalFabricRef.Namespace, gt.Spec.InternalFabricRef.Name, err)
	}

	// InternalFabric is being deleted: clean up the geneve interface now.
	// GeneveTunnel has no finalizer, so once the InternalFabric controller deletes the tunnel objects,
	// the geneveTunnel will be deleted immediately and we cannot delete the Geneve interface.
	// But if we reconciled from deletion event on the InternalFabric, chances are that InternalFabric
	// still has its finalizer at this point, so we can get the resource and delete the interface.
	// Disclaimer: this is a best-effort attempt as we have to reconcile before the InternalFabric controller
	// cleanup its finalizer. For a more consistent outcome, we rely on the geneve deletion routine.
	if !internalfabric.DeletionTimestamp.IsZero() {
		r.stopSender(gt.Name)
		if err := geneve.EnsureGeneveInterfaceAbsence(internalfabric.Spec.Interface.Node.Name); err != nil {
			klog.Warningf("Unable to delete geneve interface for genevetunnel %s: %v", req, err)
			return ctrl.Result{}, nil
		}
		klog.Infof("deleted geneve interface %s for genevetunnel %s", internalfabric.Spec.Interface.Node.Name, req)
		return ctrl.Result{}, nil
	}

	var internalnode networkingv1beta1.InternalNode
	if err := r.Get(ctx, types.NamespacedName{Name: gt.Spec.InternalNodeRef.Name}, &internalnode); err != nil {
		return ctrl.Result{}, fmt.Errorf("getting internalnode %q: %w", gt.Spec.InternalNodeRef.Name, err)
	}

	if err := geneve.EnsureGeneveInterfacePresence(
		internalfabric.Spec.Interface.Node.Name,
		internalnode.Spec.Interface.Node.IP.String(),
		internalfabric.Spec.GatewayIP.String(),
		gt.Spec.ID,
		r.Options.DisableARP,
		internalfabric.Spec.MTU,
		r.Options.GenevePort,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensuring the geneve interface presence: %w", err)
	}

	klog.Infof("Enforced interface %s for genevetunnel %s", internalfabric.Spec.Interface.Node.Name, req.String())

	if r.Options.ConnCheckOptions.PingEnabled {
		bindIP := r.Options.ConnCheckOptions.PingBindIP
		if bindIP == "" {
			bindIP = internalnode.Spec.Interface.Node.IP.String()
		}

		if err := r.initConnCheckerReceiver(context.Background(), bindIP); err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing conncheck receiver: %w", err)
		}

		cc := r.connChecker.Load()

		if err := r.ensureSender(cc, req.NamespacedName, gt, &internalfabric, &internalnode); err != nil {
			return ctrl.Result{}, err
		}

		status, err := cc.GetStatus(gt.Name)
		var (
			latency         time.Duration
			connStatusValue = networkingv1beta1.ConnectionError
		)
		if err == nil {
			latency = status.Latency
			if status.Connected {
				connStatusValue = networkingv1beta1.Connected
			}
			klog.V(6).Infof("genevetunnel %q status: connected=%v latency=%s", req.NamespacedName, status.Connected, latency)
		}

		if err := r.updateGeneveTunnelStatus(ctx, gt, connStatusValue, latency, time.Now()); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update genevetunnel status: %w", err)
		}

		// Requeue periodically to flush in-memory latency to the CR.
		return ctrl.Result{RequeueAfter: r.Options.ConnCheckOptions.PingUpdateStatusInterval}, nil
	}

	return ctrl.Result{}, nil
}

// ensureSender adds the conncheck sender for the given GeneveTunnel if it isn't already
// running. It is a no-op after the first successful call.
func (r *GeneveTunnelReconciler) ensureSender(cc *conncheck.ConnChecker, key types.NamespacedName,
	gt *networkingv1beta1.GeneveTunnel, internalfabric *networkingv1beta1.InternalFabric, internalnode *networkingv1beta1.InternalNode) error {
	if cc.HasSender(gt.Name) {
		return nil
	}

	observer := onTransition(observeGeneveLatency(internalfabric, gt), r.enqueueTransition(key))
	if err := cc.AddSender(context.Background(), gt.Name, internalnode.Spec.Interface.Node.IP.String(), observer); err != nil {
		var dupErr *conncheck.DuplicateError
		if !errors.As(err, &dupErr) {
			return fmt.Errorf("unable to add conncheck sender: %w", err)
		}
		// Sender already added concurrently — nothing else to do.
		return nil
	}

	go cc.RunSender(gt.Name)
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

func (r *GeneveTunnelReconciler) enqueueTransition(key types.NamespacedName) func() {
	return func() {
		select {
		case r.transitions <- event.GenericEvent{
			Object: &networkingv1beta1.GeneveTunnel{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
			}}:
		default:
		}
	}
}

// SetupWithManager registers the GeneveTunnelReconciler to the manager.
func (r *GeneveTunnelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	nodeSelector, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			consts.InternalNodeName: r.Options.NodeName,
		},
	})
	if err != nil {
		return fmt.Errorf("creating label selector predicate: %w", err)
	}

	internalNodePredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetName() == r.Options.NodeName
	})

	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlInternalFabricFabric).
		For(&networkingv1beta1.GeneveTunnel{},
			builder.WithPredicates(nodeSelector)).
		WatchesRawSource(source.Channel(r.transitions, &handler.EnqueueRequestForObject{})).
		Watches(&networkingv1beta1.InternalNode{},
			handler.EnqueueRequestsFromMapFunc(r.internalNodeEnqueuer),
			builder.WithPredicates(internalNodePredicate)).
		Watches(&networkingv1beta1.InternalFabric{},
			handler.EnqueueRequestsFromMapFunc(r.internalFabricEnqueuer)).
		Complete(r)
}

func (r *GeneveTunnelReconciler) internalNodeEnqueuer(ctx context.Context, _ client.Object) []reconcile.Request {
	list, err := getters.ListGeneveTunnelsByLabels(ctx, r.Client, corev1.NamespaceAll, labels.SelectorFromSet(labels.Set{
		consts.InternalNodeName: r.Options.NodeName,
	}))
	if err != nil {
		klog.Errorf("Failed to list genevetunnels for internalnode %s: %v", r.Options.NodeName, err)
		return nil
	}

	return geneveTunnelListToRequests(list)
}

func (r *GeneveTunnelReconciler) internalFabricEnqueuer(ctx context.Context, obj client.Object) []reconcile.Request {
	list, err := getters.ListGeneveTunnelsByLabels(ctx, r.Client, obj.GetNamespace(), labels.SelectorFromSet(labels.Set{
		consts.InternalFabricName: obj.GetName(),
		consts.InternalNodeName:   r.Options.NodeName,
	}))
	if err != nil {
		klog.Errorf("Failed to list genevetunnels for internalfabric %s/%s: %v", obj.GetNamespace(), obj.GetName(), err)
		return nil
	}

	return geneveTunnelListToRequests(list)
}

func geneveTunnelListToRequests(list *networkingv1beta1.GeneveTunnelList) []reconcile.Request {
	requests := make([]reconcile.Request, len(list.Items))
	for i := range list.Items {
		requests[i] = reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      list.Items[i].Name,
				Namespace: list.Items[i].Namespace,
			},
		}
	}
	return requests
}

// updateGeneveTunnelStatus updates the status of a GeneveTunnel.
func (r *GeneveTunnelReconciler) updateGeneveTunnelStatus(ctx context.Context, gt *networkingv1beta1.GeneveTunnel,
	value networkingv1beta1.ConnectionStatusValue, latency time.Duration, timestamp time.Time) error {
	if gt.Status.Value != value {
		klog.Infof("changing genevetunnel %q status to %q", client.ObjectKeyFromObject(gt), value)
	}
	gt.Status.Latency = networkingv1beta1.ConnectionLatency{
		Value:     timeutils.FormatLatency(latency),
		Timestamp: metav1.NewTime(timestamp),
	}
	gt.Status.Value = value
	if err := r.Status().Update(ctx, gt); err != nil {
		return fmt.Errorf("unable to update genevetunnel %q status: %w", client.ObjectKeyFromObject(gt), err)
	}
	return nil
}

// stopSender removes the conncheck sender for the given genevetunnel when it is deleted.
func (r *GeneveTunnelReconciler) stopSender(clusterID string) {
	cc := r.connChecker.Load()
	if cc == nil {
		return
	}
	cc.DelAndStopSender(clusterID)
}
