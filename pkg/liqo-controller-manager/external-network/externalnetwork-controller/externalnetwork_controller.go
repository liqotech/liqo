// Copyright 2019-2023 The Liqo Authors
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

package externalnetworkcontroller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayclient"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/gatewayserver"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/publickey"
)

// ExternalNetworkReconciler manage ExternalNetwork lifecycle.
type ExternalNetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	KubeClient    kubernetes.Interface
	LiqoNamespace string
	HomeCluster   *discoveryv1alpha1.ClusterIdentity

	ServiceType corev1.ServiceType
	Port        int32
	MTU         int
	Proxy       bool
}

// NewExternalNetworkReconciler returns a new ExternalNetworkReconciler.
func NewExternalNetworkReconciler(cl client.Client, s *runtime.Scheme,
	kubeClient kubernetes.Interface, liqoNamespace string,
	homeCluster *discoveryv1alpha1.ClusterIdentity,
	serviceType corev1.ServiceType, port int32, mtu int, proxy bool) *ExternalNetworkReconciler {
	return &ExternalNetworkReconciler{
		Client: cl,
		Scheme: s,

		KubeClient:    kubeClient,
		LiqoNamespace: liqoNamespace,
		HomeCluster:   homeCluster,

		ServiceType: serviceType,
		Port:        port,
		MTU:         mtu,
		Proxy:       proxy,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=networking.liqo.io,resources=externalnetworks,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=externalnetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients,verbs=get;list;watch;delete;create;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=publickeies,verbs=get;list;watch;delete;create;update;patch

// Reconcile manage ExternalNetwork lifecycle.
func (r *ExternalNetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	extNet := &networkingv1alpha1.ExternalNetwork{}
	if err = r.Get(ctx, req.NamespacedName, extNet); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		klog.Errorf("Unable to get the ExternalNetwork %q: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	defer func() {
		status := extNet.Status.DeepCopy()
		newErr := r.Update(ctx, extNet)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the ExternalNetwork %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the ExternalNetwork %q: %s", req.NamespacedName, newErr)
			err = newErr
			return
		}

		extNet.Status = *status

		newErr = r.Status().Update(ctx, extNet)
		if newErr != nil {
			if err != nil {
				klog.Errorf("Error reconciling the ExternalNetwork %q: %s", req.NamespacedName, err)
			}
			klog.Errorf("Unable to update the ExternalNetwork %q: %s", req.NamespacedName, newErr)
			err = newErr
		}
	}()

	if reflection.IsReplicated(extNet) {
		err = r.handleRemoteExternalNetwork(ctx, extNet)
	} else {
		err = r.handleLocalExternalNetwork(ctx, extNet)
	}
	return ctrl.Result{}, nil
}

func (r *ExternalNetworkReconciler) handleLocalExternalNetwork(ctx context.Context,
	extNet *networkingv1alpha1.ExternalNetwork) error {
	var err error
	if extNet.Status.Configuration == nil {
		err = fmt.Errorf("no Configuration found ExternalNetwork %q status, requeuing", extNet.Name)
		return err
	}

	if extNet.Status.ClusterIdentity == nil {
		return fmt.Errorf("no ClusterIdentity found for ExternalNetwork %q status", extNet.Name)
	}

	if err = r.ensureConfiguration(ctx, extNet, true); err != nil {
		return err
	}

	var gw *networkingv1alpha1.GatewayServer
	if gw, err = r.ensureGatewayServer(ctx, extNet); err != nil {
		return err
	}

	var ep *networkingv1alpha1.EndpointStatus
	if ep, err = r.getServerEndpoint(ctx, gw); err != nil {
		return err
	}
	extNet.Spec.ServerEndpoint = ep

	if gw.Status.SecretRef == nil {
		return fmt.Errorf("no SecretRef found for GatewayServer %q", gw.Name)
	}
	var pubKey []byte
	if pubKey, err = publickey.ExtractKeyFromSecretRef(ctx, r.Client, gw.Status.SecretRef); err != nil {
		return err
	}
	extNet.Spec.PublicKey = pubKey

	return r.ensurePublicKey(ctx, extNet, true)
}

func (r *ExternalNetworkReconciler) handleRemoteExternalNetwork(ctx context.Context,
	extNet *networkingv1alpha1.ExternalNetwork) error {
	extNet.Status.ClusterIdentity = r.HomeCluster.DeepCopy()

	cnf, err := configuration.ForgeConfigurationForRemoteCluster(ctx, r.Client, extNet.Namespace, r.LiqoNamespace)
	if err != nil {
		klog.Errorf("Unable to forge the local configuration: %s", err)
		return err
	}

	extNet.Status.Configuration = &cnf.Spec

	if extNet.Spec.ClusterIdentity == nil {
		return fmt.Errorf("no ClusterIdentity found for ExternalNetwork %q spec", extNet.Name)
	}

	if err = r.ensureConfiguration(ctx, extNet, false); err != nil {
		return err
	}

	var gw *networkingv1alpha1.GatewayClient
	if gw, err = r.ensureGatewayClient(ctx, extNet); err != nil {
		return err
	}

	if gw.Status.SecretRef == nil {
		return fmt.Errorf("no SecretRef found for GatewayClient %q", gw.Name)
	}
	var pubKey []byte
	if pubKey, err = publickey.ExtractKeyFromSecretRef(ctx, r.Client, gw.Status.SecretRef); err != nil {
		return err
	}
	extNet.Status.PublicKey = pubKey

	return r.ensurePublicKey(ctx, extNet, false)
}

func (r *ExternalNetworkReconciler) ensurePublicKey(ctx context.Context,
	extNet *networkingv1alpha1.ExternalNetwork, isLocal bool) error {
	if isLocal {
		if extNet.Status.PublicKey == nil || len(extNet.Status.PublicKey) == 0 {
			return fmt.Errorf("no PublicKey found ExternalNetwork %q status", extNet.Name)
		}
	} else {
		if extNet.Spec.PublicKey == nil || len(extNet.Spec.PublicKey) == 0 {
			return fmt.Errorf("no PublicKey found ExternalNetwork %q spec", extNet.Name)
		}
	}

	var clusterID string
	if extNet.Labels != nil {
		if isLocal {
			if v, ok := extNet.Labels[consts.ReplicationDestinationLabel]; ok {
				clusterID = v
			}
		} else {
			if v, ok := extNet.Labels[consts.ReplicationOriginLabel]; ok {
				clusterID = v
			}
		}
	}
	if clusterID == "" {
		return fmt.Errorf("no remote ClusterID found for ExternalNetwork %q", extNet.Name)
	}

	var key []byte
	if isLocal {
		key = extNet.Status.PublicKey
	} else {
		key = extNet.Spec.PublicKey
	}

	pubKey, err := publickey.ForgePublicKey(publickey.DefaultPublicKeyName(getIdentity(extNet, isLocal)), extNet.Namespace, clusterID, key)
	if err != nil {
		return err
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, pubKey, func() error {
		if err = publickey.MutatePublicKey(pubKey, clusterID, key); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(extNet, pubKey, r.Scheme)
	}); err != nil {
		return err
	}

	return nil
}

func (r *ExternalNetworkReconciler) ensureGatewayClient(ctx context.Context,
	extNet *networkingv1alpha1.ExternalNetwork) (*networkingv1alpha1.GatewayClient, error) {
	if extNet.Spec.ServerEndpoint == nil {
		return nil, fmt.Errorf("no remote ServerEndpoint found for ExternalNetwork %q", extNet.Name)
	}

	var remoteClusterID string
	if extNet.Labels != nil {
		if v, ok := extNet.Labels[consts.ReplicationOriginLabel]; ok {
			remoteClusterID = v
		}
	}
	if remoteClusterID == "" {
		return nil, fmt.Errorf("no remote ClusterID found for ExternalNetwork %q", extNet.Name)
	}

	opts := &gatewayclient.ForgeOptions{
		KubeClient:        r.KubeClient,
		RemoteClusterID:   remoteClusterID,
		GatewayType:       gatewayclient.DefaultGatewayType,
		TemplateName:      gatewayclient.DefaultTemplateName,
		TemplateNamespace: r.LiqoNamespace,
		MTU:               r.MTU,
		Addresses:         extNet.Spec.ServerEndpoint.Addresses,
		Port:              extNet.Spec.ServerEndpoint.Port,
	}
	if extNet.Spec.ServerEndpoint.Protocol != nil {
		opts.Protocol = string(*extNet.Spec.ServerEndpoint.Protocol)
	} else {
		opts.Protocol = string(corev1.ProtocolTCP)
	}
	gwClient, err := gatewayclient.ForgeGatewayClient(gatewayclient.DefaultGatewayClientName(getIdentity(extNet, false)), extNet.Namespace, opts)
	if err != nil {
		return nil, err
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, gwClient, func() error {
		if err = gatewayclient.MutateGatewayClient(gwClient, opts); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(extNet, gwClient, r.Scheme)
	}); err != nil {
		return nil, err
	}
	return gwClient, nil
}

func (r *ExternalNetworkReconciler) getServerEndpoint(ctx context.Context,
	gw *networkingv1alpha1.GatewayServer) (*networkingv1alpha1.EndpointStatus, error) {
	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      gw.Name,
		Namespace: gw.Namespace,
	}, gw); err != nil {
		return nil, err
	}

	ep := gw.Status.Endpoint
	if ep == nil {
		return nil, fmt.Errorf("no Endpoint found for GatewayServer %q", gw.Name)
	}
	return ep, nil
}

func (r *ExternalNetworkReconciler) ensureGatewayServer(ctx context.Context,
	extNet *networkingv1alpha1.ExternalNetwork) (*networkingv1alpha1.GatewayServer, error) {
	var remoteClusterID string
	if extNet.Labels != nil {
		if v, ok := extNet.Labels[consts.ReplicationDestinationLabel]; ok {
			remoteClusterID = v
		}
	}
	if remoteClusterID == "" {
		return nil, fmt.Errorf("no remote ClusterID found for ExternalNetwork %q", extNet.Name)
	}

	opts := &gatewayserver.ForgeOptions{
		KubeClient:        r.KubeClient,
		RemoteClusterID:   remoteClusterID,
		GatewayType:       gatewayserver.DefaultGatewayType,
		TemplateName:      gatewayserver.DefaultTemplateName,
		TemplateNamespace: r.LiqoNamespace,
		ServiceType:       r.ServiceType,
		MTU:               r.MTU,
		Port:              r.Port,
		Proxy:             r.Proxy,
	}
	gwServer, err := gatewayserver.ForgeGatewayServer(gatewayserver.DefaultGatewayServerName(getIdentity(extNet, true)), extNet.Namespace, opts)
	if err != nil {
		return nil, err
	}

	if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, gwServer, func() error {
		if err = gatewayserver.MutateGatewayServer(gwServer, opts); err != nil {
			return err
		}
		return controllerutil.SetControllerReference(extNet, gwServer, r.Scheme)
	}); err != nil {
		return nil, err
	}
	return gwServer, nil
}

func (r *ExternalNetworkReconciler) ensureConfiguration(ctx context.Context, extNet *networkingv1alpha1.ExternalNetwork, isLocal bool) error {
	cnf := &networkingv1alpha1.Configuration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configuration.DefaultConfigurationName(getIdentity(extNet, isLocal)),
			Namespace: extNet.Namespace,
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, cnf, func() error {
		if cnf.Labels == nil {
			cnf.Labels = make(map[string]string)
		}
		if extNet.Labels != nil {
			if isLocal {
				if v, ok := extNet.Labels[consts.ReplicationDestinationLabel]; ok {
					cnf.Labels[consts.RemoteClusterID] = v
				}
			} else {
				if v, ok := extNet.Labels[consts.ReplicationOriginLabel]; ok {
					cnf.Labels[consts.RemoteClusterID] = v
				}
			}
		}

		if isLocal {
			cnf.Spec.Remote = extNet.Status.Configuration.Remote
		} else {
			cnf.Spec.Remote = extNet.Spec.Configuration.Remote
		}
		return controllerutil.SetControllerReference(extNet, cnf, r.Scheme)
	}); err != nil {
		klog.Errorf("Unable to create or update the Configuration %q: %s", cnf.Name, err)
		return err
	}

	return nil
}

func getIdentity(extNet *networkingv1alpha1.ExternalNetwork, isLocal bool) *discoveryv1alpha1.ClusterIdentity {
	if isLocal {
		return extNet.Status.ClusterIdentity
	}
	return extNet.Spec.ClusterIdentity
}

// SetupWithManager register the ExternalNetworkReconciler to the manager.
func (r *ExternalNetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Owns(&networkingv1alpha1.Configuration{}).
		Owns(&networkingv1alpha1.GatewayServer{}).
		Owns(&networkingv1alpha1.GatewayClient{}).
		Owns(&networkingv1alpha1.PublicKey{}).
		For(&networkingv1alpha1.ExternalNetwork{}).
		Complete(r)
}
