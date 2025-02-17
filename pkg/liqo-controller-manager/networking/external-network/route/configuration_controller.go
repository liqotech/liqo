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

package route

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	configuration "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/external-network/configuration"
	"github.com/liqotech/liqo/pkg/utils"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// ConfigurationReconciler manage Configuration.
type ConfigurationReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder
}

// NewConfigurationReconciler returns a new ConfigurationReconciler.
func NewConfigurationReconciler(cl client.Client, s *runtime.Scheme,
	er record.EventRecorder) *ConfigurationReconciler {
	return &ConfigurationReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=routeconfigurations,verbs=get;list;watch;update;patch;create;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalnodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients,verbs=get;list;watch

// Reconcile manage Configurations.
func (r *ConfigurationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	conf := &networkingv1beta1.Configuration{}
	if err = r.Get(ctx, req.NamespacedName, conf); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("There is no configuration %s", req.String())
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the configuration %q: %w", req.NamespacedName, err)
	}

	klog.V(4).Infof("Reconciling configuration %s", req.String())

	return ctrl.Result{}, enforceRouteConfigurationPresence(ctx, r.Client, r.Scheme, conf)
}

// SetupWithManager register the ConfigurationReconciler to the manager.
// We need to watch GatewayServer and GatewayClient to trigger the reconcile of the Configuration
// when we are aware if the gateway pod will act as a server or a client.
// This allows us to set the correct gateway IP inside the route.
func (r *ConfigurationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			configuration.Configured: configuration.ConfiguredValue,
		},
	})
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlConfigurationRoute).
		For(&networkingv1beta1.Configuration{}, builder.WithPredicates(p)).
		Watches(
			&networkingv1beta1.GatewayServer{},
			handler.EnqueueRequestsFromMapFunc(r.configurationEnqueuerByRemoteID()),
		).
		Watches(
			&networkingv1beta1.GatewayClient{},
			handler.EnqueueRequestsFromMapFunc(r.configurationEnqueuerByRemoteID()),
		).
		Complete(r)
}

func (r *ConfigurationReconciler) configurationEnqueuerByRemoteID() handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		labels := obj.GetLabels()
		if labels == nil {
			klog.Errorf("unable to get the labels of gateway %s", obj.GetName())
			return nil
		}
		remoteID, ok := utils.GetClusterIDFromLabels(labels)
		if !ok {
			klog.Errorf("unable to get the remote cluster ID from the labels of gateway %s", obj.GetName())
			return nil
		}
		cfg, err := getters.GetConfigurationByClusterID(ctx, r.Client, remoteID, corev1.NamespaceAll)
		if err != nil {
			klog.Errorf("unable to get the configuration for cluster %s: %s", remoteID, err)
			return nil
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(cfg)}}
	}
}
