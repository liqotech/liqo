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

package servercontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	internalnetwork "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network/fabricipam"
	"github.com/liqotech/liqo/pkg/utils"
	cidrutils "github.com/liqotech/liqo/pkg/utils/cidr"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// ServerReconciler manage GatewayServer lifecycle.
type ServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// NewServerReconciler returns a new ServerReconciler.
func NewServerReconciler(cl client.Client, s *runtime.Scheme) *ServerReconciler {
	return &ServerReconciler{
		Client: cl,
		Scheme: s,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=internalfabrics,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage GatewayServer lifecycle.
func (r *ServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	gwServer := &networkingv1beta1.GatewayServer{}
	if err = r.Get(ctx, req.NamespacedName, gwServer); err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("Gateway server %q not found", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the gateway server %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	ipam, err := fabricipam.Get(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to initialize the IPAM: %w", err)
	}

	remoteClusterID, ok := utils.GetClusterIDFromLabels(gwServer.Labels)
	if !ok {
		err = fmt.Errorf("remote cluster ID not found in the gateway server %q", req.NamespacedName)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	configuration, err := getters.GetConfigurationByClusterID(ctx, r.Client, remoteClusterID, corev1.NamespaceAll)
	if err != nil {
		klog.Errorf("Unable to get the configuration for the remote cluster %q: %s", remoteClusterID, err)
		return ctrl.Result{}, err
	}

	if err = r.ensureInternalFabric(ctx, gwServer, configuration, remoteClusterID, ipam); err != nil {
		klog.Errorf("Unable to ensure the internal fabric for the gateway server %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// ensureInternalFabric ensures the InternalFabric is correctly configured.
func (r *ServerReconciler) ensureInternalFabric(ctx context.Context, gwServer *networkingv1beta1.GatewayServer,
	configuration *networkingv1beta1.Configuration, remoteClusterID liqov1beta1.ClusterID, ipam *fabricipam.IPAM) error {
	if configuration.Status.Remote == nil {
		return fmt.Errorf("remote configuration not found for the gateway server %q", gwServer.Name)
	}
	if gwServer.Status.InternalEndpoint == nil || gwServer.Status.InternalEndpoint.IP == nil {
		return fmt.Errorf("internal endpoint not found for the gateway server %q", gwServer.Name)
	}

	internalFabric := &networkingv1beta1.InternalFabric{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwServer.Name,
			Namespace: gwServer.Namespace,
		},
	}
	if _, err := resource.CreateOrUpdate(ctx, r.Client, internalFabric, func() error {
		var err error
		if internalFabric.Labels == nil {
			internalFabric.Labels = make(map[string]string)
		}
		internalFabric.Labels[consts.RemoteClusterID] = string(remoteClusterID)

		internalFabric.Spec.MTU = gwServer.Spec.MTU

		internalFabric.Spec.GatewayIP = *gwServer.Status.InternalEndpoint.IP

		if internalFabric.Spec.Interface.Node.Name, err = internalnetwork.FindFreeInterfaceName(ctx, r.Client, internalFabric); err != nil {
			return err
		}

		ip, err := ipam.Allocate(internalFabric.GetName())
		if err != nil {
			return err
		}
		internalFabric.Spec.Interface.Gateway.IP = networkingv1beta1.IP(ip.String())

		internalFabric.Spec.RemoteCIDRs = []networkingv1beta1.CIDR{
			*cidrutils.GetPrimary(configuration.Status.Remote.CIDR.Pod),
			*cidrutils.GetPrimary(configuration.Status.Remote.CIDR.External),
		}

		return controllerutil.SetControllerReference(gwServer, internalFabric, r.Scheme)
	}); err != nil {
		return err
	}

	return nil
}

// SetupWithManager register the ServerReconciler to the manager.
func (r *ServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlGatewayServerInternal).
		Owns(&networkingv1beta1.InternalFabric{}).
		For(&networkingv1beta1.GatewayServer{}).
		Complete(r)
}
