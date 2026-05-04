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

package geneve

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/connection/conncheck"
	"github.com/liqotech/liqo/pkg/gateway/fabric"
	geneveutils "github.com/liqotech/liqo/pkg/utils/network/geneve"
)

// InternalNodeReconciler manage InternalNode.
type InternalNodeReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	Options        *fabric.Options

	connChecker   *conncheck.ConnChecker
	connCheckerMu sync.Mutex
	// tunnelNames maps internalnode.Name to the GeneveTunnel name, used for cleanup on deletion.
	tunnelNames   map[string]string
	tunnelNamesMu sync.RWMutex
}

// NewInternalNodeReconciler returns a new InternalNodeReconciler.
func NewInternalNodeReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, opts *fabric.Options) (*InternalNodeReconciler, error) {
	return &InternalNodeReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        opts,
		tunnelNames:    make(map[string]string),
	}, nil
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=genevetunnels/status,verbs=get;update;patch

// Reconcile manage InternalNodes.
func (r *InternalNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	internalnode := &networkingv1beta1.InternalNode{}
	if err = r.Get(ctx, req.NamespacedName, internalnode); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("There is no internalnode %s", req.String())
			r.stopSender(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the internalnode %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling internalnode %s", req.String())

	internalFabric, err := getInternalFabric(ctx, r.Client, r.Options.GwOptions.Name, r.Options.GwOptions.RemoteClusterID, r.Options.GwOptions.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to get the internal fabric: %w", err)
	}

	id, err := geneveutils.GetGeneveTunnelID(ctx, r.Client, internalFabric.Name, internalnode.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("waiting for geneve tunnel (internalfabric %s, internalnode %s) to be created...", internalFabric.Name, internalnode.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("getting geneve tunnel (internalfabric %s, internalnode %s): %w", internalFabric.Name, internalnode.Name, err)
	}

	var remoteIP *networkingv1beta1.IP
	switch {
	case r.Options.GwOptions.NodeName == internalnode.Name:
		remoteIP = internalnode.Status.NodeIP.Local
	default:
		remoteIP = internalnode.Status.NodeIP.Remote
	}

	if remoteIP == nil {
		klog.Infof("waiting for remote IP of internalnode %s to be set...", internalnode.Name)
		return ctrl.Result{}, nil
	}

	if err := geneveutils.EnsureGeneveInterfacePresence(
		internalnode.Spec.Interface.Gateway.Name,
		internalFabric.Spec.Interface.Gateway.IP.String(),
		remoteIP.String(),
		id,
		r.Options.DisableARP,
		internalFabric.Spec.MTU,
		r.Options.GenevePort,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to ensure the geneve interface presence: %w", err)
	}

	klog.Infof("Enforced interface %s for internalnode %s", internalnode.Spec.Interface.Gateway.Name, internalnode.Name)

	if internalnode.Spec.Interface.Node.IP == "" {
		klog.Infof("waiting for inner IP of internalnode %s to be set...", internalnode.Name)
		return ctrl.Result{}, nil
	}

	tunnelName := fmt.Sprintf("%s-%s", internalFabric.Name, internalnode.Name)
	r.tunnelNamesMu.Lock()
	r.tunnelNames[internalnode.Name] = tunnelName
	r.tunnelNamesMu.Unlock()

	updateCallback := ForgeUpdateGeneveTunnelCallback(ctx, r.Client, r.Options, tunnelName, internalFabric.Namespace)

	switch r.Options.PingEnabled {
	case true:
		cc, err := r.getOrInitConnChecker(ctx, internalFabric.Spec.Interface.Gateway.IP.String())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("initializing conncheck: %w", err)
		}

		err = cc.AddSender(ctx, tunnelName, internalnode.Spec.Interface.Node.IP.String(), updateCallback)
		if err != nil {
			switch err.(type) {
			case *conncheck.DuplicateError:
				return ctrl.Result{}, nil
			default:
				return ctrl.Result{}, fmt.Errorf("unable to add conncheck sender: %w", err)
			}
		}

		go cc.RunSender(tunnelName)

	case false:
		if err := updateCallback(true, 0, time.Time{}); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to update genevetunnel status: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the InternalNodeReconciler to the manager.
func (r *InternalNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlInternalNodeGeneve).
		For(&networkingv1beta1.InternalNode{}).
		Watches(&networkingv1beta1.GeneveTunnel{}, handler.EnqueueRequestsFromMapFunc(geneveToInternalNodeEnqueuer)).
		Complete(r)
}

func geneveToInternalNodeEnqueuer(_ context.Context, obj client.Object) []reconcile.Request {
	v, ok := obj.GetLabels()[consts.InternalNodeName]
	if !ok {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: v}}}
}

// getOrInitConnChecker lazily creates the ConnChecker bound to the gateway inner IP on first call.
func (r *InternalNodeReconciler) getOrInitConnChecker(ctx context.Context, gatewayIP string) (*conncheck.ConnChecker, error) {
	r.connCheckerMu.Lock()
	defer r.connCheckerMu.Unlock()
	if r.connChecker != nil {
		return r.connChecker, nil
	}
	opts := *r.Options.ConnCheckOptions
	opts.BindIP = gatewayIP
	cc, err := conncheck.NewConnChecker(&opts)
	if err != nil {
		return nil, fmt.Errorf("creating geneve conncheck: %w", err)
	}
	go cc.RunReceiver(ctx)
	go cc.RunReceiverDisconnectObserver(ctx)
	r.connChecker = cc
	klog.Infof("geneve conncheck started, bound to %s:%d", gatewayIP, opts.PingPort)
	return cc, nil
}

// stopSender removes the conncheck sender for the given internalnode when it is deleted.
func (r *InternalNodeReconciler) stopSender(internalNodeName string) {
	r.connCheckerMu.Lock()
	cc := r.connChecker
	r.connCheckerMu.Unlock()
	if cc == nil {
		return
	}
	r.tunnelNamesMu.Lock()
	tunnelName, ok := r.tunnelNames[internalNodeName]
	delete(r.tunnelNames, internalNodeName)
	r.tunnelNamesMu.Unlock()
	if ok {
		cc.DelAndStopSender(tunnelName)
	}
}
