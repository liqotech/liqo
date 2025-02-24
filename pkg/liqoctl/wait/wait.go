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

package wait

import (
	"context"
	"fmt"
	"time"

	"github.com/pterm/pterm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	authgetters "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/getters"
	networkingutils "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/utils"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreigncluster"
	getters "github.com/liqotech/liqo/pkg/utils/getters"
)

// Waiter is a struct that contains the necessary information to wait for resource events.
type Waiter struct {
	// Printer is the object used to output messages in the appropriate format.
	Printer *output.Printer
	// crClient is the controller runtime client.
	CRClient client.Client
	// kubeClient is a Kubernetes clientset for interacting with the base Kubernetes APIs.
	KubeClient kubernetes.Interface
}

// NewWaiterFromFactory creates a new Waiter object from the given factory.
func NewWaiterFromFactory(f *factory.Factory) *Waiter {
	return &Waiter{
		Printer:    f.Printer,
		CRClient:   f.CRClient,
		KubeClient: f.KubeClient,
	}
}

// ForNetwork waits until the networking has been established with the remote cluster or the timeout expires.
func (w *Waiter) ForNetwork(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	remName := remoteClusterID
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for network to the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsNetworkingEstablishedOrDisabled, 1*time.Second)
	if err != nil {
		s.Fail(fmt.Sprintf("Failed establishing networking to the remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Network established to the remote cluster %q", remName))
	return nil
}

// ForResourceSliceAuthentication waits until the ResourceSlice authentication has been accepted or the timeout expires.
func (w *Waiter) ForResourceSliceAuthentication(ctx context.Context, resourceSlice *authv1beta1.ResourceSlice) error {
	s := w.Printer.StartSpinner("Waiting for ResourceSlice authentication to be accepted")

	nsName := client.ObjectKeyFromObject(resourceSlice)
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		if err := w.CRClient.Get(ctx, nsName, resourceSlice); err != nil {
			return false, client.IgnoreNotFound(err)
		}

		authCondition := authentication.GetCondition(resourceSlice, authv1beta1.ResourceSliceConditionTypeAuthentication)
		if authCondition != nil && authCondition.Status == authv1beta1.ResourceSliceConditionAccepted {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for ResourceSlice authentication to be accepted: %s", output.PrettyErr(err)))
		return err
	}

	s.Success("ResourceSlice authentication: ", authv1beta1.ResourceSliceConditionAccepted)
	return nil
}

// ForNodeReady waits until the node has been added to the cluster and is Ready, or the timeout expires.
func (w *Waiter) ForNodeReady(ctx context.Context, nodeName string) error {
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for node %s to be Ready", nodeName))

	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		var node corev1.Node
		if err := w.CRClient.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
			return false, client.IgnoreNotFound(err)
		}

		if !utils.IsNodeReady(&node) {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for node %s to be Ready: %s", nodeName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Node %s is Ready", nodeName))
	return nil
}

// ForOffloading waits until the status on the NamespaceOffloading resource states that the offloading has been successfully
// established or the timeout expires.
func (w *Waiter) ForOffloading(ctx context.Context, namespace string) error {
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for offloading of namespace %q to complete", namespace))
	noClusterSelected := false
	var offload *offloadingv1beta1.NamespaceOffloading
	err := wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true, func(ctx context.Context) (done bool, err error) {
		offload, err = getters.GetOffloadingByNamespace(ctx, w.CRClient, namespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		// Retry in case the observed generation does not match, as the status still needs to be updated.
		if offload.Status.ObservedGeneration != offload.GetGeneration() {
			return false, nil
		}

		someFailed := offload.Status.OffloadingPhase == offloadingv1beta1.SomeFailedOffloadingPhaseType
		allFailed := offload.Status.OffloadingPhase == offloadingv1beta1.AllFailedOffloadingPhaseType
		if someFailed || allFailed {
			return true, fmt.Errorf("the offloading is in %q state", offload.Status.OffloadingPhase)
		}

		ready := offload.Status.OffloadingPhase == offloadingv1beta1.ReadyOffloadingPhaseType
		noClusterSelected = offload.Status.OffloadingPhase == offloadingv1beta1.NoClusterSelectedOffloadingPhaseType

		return ready || noClusterSelected, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for offloading to complete: %s", output.PrettyErr(err)))
		return err
	}
	if noClusterSelected {
		s.Warning("Offloading completed, but no cluster was selected")
		return nil
	}
	s.Success("Offloading completed successfully")
	return nil
}

// ForUnoffloading waits until the status on the NamespaceOffloading resource states that the offloading has been
// successfully removed or the timeout expires.
func (w *Waiter) ForUnoffloading(ctx context.Context, namespace string) error {
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for unoffloading of namespace %q to complete", namespace))
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		_, err = getters.GetOffloadingByNamespace(ctx, w.CRClient, namespace)
		return apierrors.IsNotFound(err), client.IgnoreNotFound(err)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for unoffloading to complete: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Unoffloading completed successfully")
	return nil
}

// ForConfiguration waits until the status on the Configuration resource states that the configuration has been
// successfully applied. If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func (w *Waiter) ForConfiguration(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, tenantNamespace string) error {
	s := w.Printer.StartSpinner("Waiting for configuration to be applied")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		conf, err := getters.GetConfigurationByClusterID(ctx, w.CRClient, remoteClusterID, tenantNamespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return networkingutils.IsConfigurationStatusSet(conf.Status), nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for configuration to be applied: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Configuration applied successfully")
	return nil
}

// ForGatewayPodReady waits until the pod of a Gateway resource has been created and is ready.
func (w *Waiter) ForGatewayPodReady(ctx context.Context, gateway client.Object) error {
	gatewayDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      forge.GatewayResourceName(gateway.GetName()),
			Namespace: gateway.GetNamespace(),
		},
	}
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for gateway pod %s to be ready", gatewayDeployment.GetName()))
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = w.CRClient.Get(ctx, client.ObjectKeyFromObject(gatewayDeployment), gatewayDeployment)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return gatewayDeployment.Status.ReadyReplicas > 0, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for gateway pod %s to be ready: %s", gatewayDeployment.GetName(), output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Gateway pod %s is ready", gatewayDeployment.GetName()))
	return nil
}

// ForGatewayServerStatusEndpoint waits until the service of a Gateway resource has been created
// (i.e., until its endpoint status is not set).
func (w *Waiter) ForGatewayServerStatusEndpoint(ctx context.Context, gwServer *networkingv1beta1.GatewayServer) error {
	s := w.Printer.StartSpinner("Waiting for gateway server Service to be created")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = w.CRClient.Get(ctx, client.ObjectKey{Name: gwServer.Name, Namespace: gwServer.Namespace}, gwServer)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return gwServer.Status.Endpoint != nil && len(gwServer.Status.Endpoint.Addresses) > 0, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for gateway server Service to be created: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Gateway server Service created successfully")
	return nil
}

// ForGatewayServerSecretRef waits until the secret containing the public key of a gateway server has been created
// (i.e., until its secret reference status is not set).
func (w *Waiter) ForGatewayServerSecretRef(ctx context.Context, gwServer *networkingv1beta1.GatewayServer) error {
	s := w.Printer.StartSpinner("Waiting for gateway server Secret to be created")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = w.CRClient.Get(ctx, client.ObjectKeyFromObject(gwServer), gwServer)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return gwServer.Status.SecretRef != nil, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for gateway server Secret to be created: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Gateway server Secret created successfully")
	return nil
}

// ForGatewayClientSecretRef waits until the secret containing the public key of a gateway client has been created
// (i.e., until its secret reference status is not set).
func (w *Waiter) ForGatewayClientSecretRef(ctx context.Context, gwClient *networkingv1beta1.GatewayClient) error {
	s := w.Printer.StartSpinner("Waiting for gateway client Secret to be created")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = w.CRClient.Get(ctx, client.ObjectKeyFromObject(gwClient), gwClient)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return gwClient.Status.SecretRef != nil, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for gateway client Secret to be created: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Gateway client Secret created successfully")
	return nil
}

// ForConnection waits until the Connection resource has been created.
func (w *Waiter) ForConnection(ctx context.Context, namespace string,
	remoteCluster liqov1beta1.ClusterID) (*networkingv1beta1.Connection, error) {
	s := w.Printer.StartSpinner("Waiting for Connection to be created")
	var conn *networkingv1beta1.Connection
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: string(remoteCluster)}.AsSelector()
		connections, err := getters.ListConnectionsByLabel(ctx, w.CRClient, namespace, remoteClusterIDSelector)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		switch len(connections.Items) {
		case 0:
			return false, nil
		case 1:
			conn = &connections.Items[0]
			return true, nil
		default:
			return false, fmt.Errorf("more than one Connection resource found for remote cluster %q", remoteCluster)
		}
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for Connection to be created: %s", output.PrettyErr(err)))
		return nil, err
	}
	s.Success("Connection created successfully")
	return conn, nil
}

// ForConnectionEstablished waits until the status of the Connection is established.
func (w *Waiter) ForConnectionEstablished(ctx context.Context, conn *networkingv1beta1.Connection) error {
	s := w.Printer.StartSpinner("Waiting for Connection status to be established")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = w.CRClient.Get(ctx, client.ObjectKeyFromObject(conn), conn)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return conn.Status.Value == networkingv1beta1.Connected, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for Connection status to be established: %s", output.PrettyErr(err)))
		return err
	}

	s.Success("Connection is established")
	return nil
}

// ForNonce waits until the secret containing the nonce has been created or the timeout expires.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func (w *Waiter) ForNonce(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, tenantNamespace string, silent bool) error {
	var s *pterm.SpinnerPrinter

	if !silent {
		s = w.Printer.StartSpinner("Waiting for nonce to be generated")
	}

	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		secret, err := getters.GetNonceSecretByClusterID(ctx, w.CRClient, remoteClusterID, tenantNamespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		if _, err := authgetters.GetNonceFromSecret(secret); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if !silent {
			s.Fail(fmt.Sprintf("Failed waiting for nonce to be generated: %s", output.PrettyErr(err)))
		}
		return err
	}

	if !silent {
		s.Success("Nonce generated successfully")
	}

	return nil
}

// ForSignedNonce waits until the signed nonce secret has been signed and returns the signature.
// If tenantNamespace is empty this function searches in all the namespaces in the cluster.
func (w *Waiter) ForSignedNonce(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, tenantNamespace string, silent bool) error {
	var s *pterm.SpinnerPrinter

	if !silent {
		s = w.Printer.StartSpinner("Waiting for nonce to be signed")
	}

	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		secret, err := getters.GetSignedNonceSecretByClusterID(ctx, w.CRClient, remoteClusterID, tenantNamespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if _, err = authgetters.GetSignedNonceFromSecret(secret); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if !silent {
			s.Fail(fmt.Sprintf("Failed waiting for nonce to be signed: %s", output.PrettyErr(err)))
		}
		return err
	}

	if !silent {
		s.Success("Nonce is signed")
	}

	return nil
}

// ForTenantStatus waits until the tenant status has been updated or the timeout expires.
func (w *Waiter) ForTenantStatus(ctx context.Context, remoteClusterID liqov1beta1.ClusterID, tenantNamespace string) error {
	s := w.Printer.StartSpinner("Waiting for tenant status to be filled")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		tenant, err := getters.GetTenantByClusterID(ctx, w.CRClient, remoteClusterID, tenantNamespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		if tenant.Status.AuthParams == nil {
			return false, nil
		}

		if tenant.Status.TenantNamespace == "" {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for tenant status to be updated: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Tenant status is filled")
	return nil
}

// ForIdentityStatus waits until the identity status has been updated or the timeout expires.
func (w *Waiter) ForIdentityStatus(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := w.Printer.StartSpinner("Waiting for identity status to be filled")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		identity, err := getters.GetControlPlaneIdentityByClusterID(ctx, w.CRClient, remoteClusterID)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		if identity.Status.KubeconfigSecretRef == nil || identity.Status.KubeconfigSecretRef.Name == "" {
			return false, nil
		}

		return true, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for identity status to be updated: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Identity status is filled")
	return nil
}

// ForTenantNamespaceAbsence waits until the tenant namespace has been deleted or the timeout expires.
func (w *Waiter) ForTenantNamespaceAbsence(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := w.Printer.StartSpinner("Waiting for tenant namespace to be deleted")
	namespaceManager := tenantnamespace.NewManager(w.KubeClient, w.CRClient.Scheme())
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		_, tmpErr := namespaceManager.GetNamespace(ctx, remoteClusterID)
		return apierrors.IsNotFound(tmpErr), client.IgnoreNotFound(tmpErr)
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for tenant namespace to be deleted: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Ensured tenant namespace absence")
	return nil
}

// ForResourceSlicesAbsence waits until the resource slices with the given selector have been deleted or the timeout expires.
func (w *Waiter) ForResourceSlicesAbsence(ctx context.Context, namespace string, selector labels.Selector) error {
	s := w.Printer.StartSpinner("Waiting for resource slices to be deleted")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		resourceSlices, err := getters.ListResourceSlicesByLabel(ctx, w.CRClient, namespace, selector)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return len(resourceSlices) == 0, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for resource slices to be deleted: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Ensured resourceslices absence")
	return nil
}

// ForVirtualNodesAbsence waits until the virtual nodes with the given selector have been deleted or the timeout expires.
func (w *Waiter) ForVirtualNodesAbsence(ctx context.Context, remoteClusterID liqov1beta1.ClusterID) error {
	s := w.Printer.StartSpinner("Waiting for virtual nodes to be deleted")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		virtualNodes, err := getters.ListVirtualNodesByClusterID(ctx, w.CRClient, remoteClusterID)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return len(virtualNodes) == 0, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for virtual nodes to be deleted: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Ensured virtualnodes absence")
	return nil
}
