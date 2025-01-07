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

package gwmasqbypass

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway"
)

var (
	checkLeftoverRulesOnce sync.Once
)

// PodReconciler manage Pod.
type PodReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
	GenericEvents  chan event.GenericEvent
	Options        *Options
}

// NewPodReconciler returns a new PodReconciler.
func NewPodReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder, options *Options) *PodReconciler {
	return &PodReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
		Options:        options,
		GenericEvents:  make(chan event.GenericEvent),
	}
}

// cluster-role
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=firewallconfigurations,verbs=get;list;watch;update;patch;create;delete

// Reconcile manage Pods.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	checkLeftoverRulesOnce.Do(func() {
		utilruntime.Must(r.CheckLeftoverRules(ctx))
	})

	var err error
	pod := &corev1.Pod{
		ObjectMeta: ctrl.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}
	if err = r.Get(ctx, req.NamespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no pod %s", req.String())
			return ctrl.Result{}, enforceFirewallPodAbsence(ctx, r.Client, r.Options, pod)
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the pod %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling pod %s", req.String())

	if pod.Spec.NodeName != "" {
		PopulatePodKeyToNodeMap(pod)
	} else {
		return ctrl.Result{}, nil
	}

	op, err := enforceFirewallPodPresence(ctx, r.Client, r.Scheme, r.Options, pod)
	if err != nil {
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		klog.Infof("Updated gw-masquerade-bypass for pod %s", req.String())
	}

	return ctrl.Result{}, nil
}

// SetupWithManager register the PodReconciler to the manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p, err := predicate.LabelSelectorPredicate(v1.LabelSelector{
		MatchLabels: gateway.ForgeActiveGatewayPodLabels(),
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlPodGwMasq).
		For(&corev1.Pod{}, builder.WithPredicates(p)).
		WatchesRawSource(NewLeftoverPodsSource(r.GenericEvents, NewLeftoverPodsEventHandler())).
		Complete(r)
}
