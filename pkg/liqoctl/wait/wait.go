// Copyright 2019-2024 The Liqo Authors
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

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/gateway/forge"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/rest/configuration"
	"github.com/liqotech/liqo/pkg/utils"
	fcutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
	getters "github.com/liqotech/liqo/pkg/utils/getters"
)

// Waiter is a struct that contains the necessary information to wait for resource events.
type Waiter struct {
	// Printer is the object used to output messages in the appropriate format.
	Printer *output.Printer
	// crClient is the controller runtime client.
	CRClient client.Client
}

// NewWaiterFromFactory creates a new Waiter object from the given factory.
func NewWaiterFromFactory(f *factory.Factory) *Waiter {
	return &Waiter{
		Printer:  f.Printer,
		CRClient: f.CRClient,
	}
}

// ForUnpeering waits until the status on the foreiglcusters resource states that the in/outgoing peering has been successfully
// set to None or the timeout expires.
func (w *Waiter) ForUnpeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Unpeering from the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsUnpeered, 1*time.Second)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("Failed unpeering from remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Successfully unpeered from remote cluster %q", remName))
	return nil
}

// ForOutgoingUnpeering waits until the status on the foreiglcusters resource states that the outgoing peering has been successfully
// set to None or the timeout expires.
func (w *Waiter) ForOutgoingUnpeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Disabling outgoing peering to the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsOutgoingPeeringNone, 1*time.Second)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("Failed disabling outgoing peering to the remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Successfully disabled outgoing peering to the remote cluster %q", remName))
	return nil
}

// ForIncomingUnpeering waits until the status on the foreiglcusters resource states that the incoming peering has been successfully
// set to None or the timeout expires.
func (w *Waiter) ForIncomingUnpeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Disabling incoming peering to the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsIncomingPeeringNo, 1*time.Second)
	if client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("Failed disabling incoming peering to the remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Successfully disabled incoming peering to the remote cluster %q", remName))
	return nil
}

// ForAuth waits until the authentication has been established with the remote cluster or the timeout expires.
func (w *Waiter) ForAuth(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for authentication to the cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsAuthenticated, 1*time.Second)
	if err != nil {
		s.Fail(fmt.Sprintf("Authentication to the remote cluster %q failed: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Authenticated to cluster %q", remName))
	return nil
}

// ForNetwork waits until the networking has been established with the remote cluster or the timeout expires.
func (w *Waiter) ForNetwork(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for network to the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsNetworkingEstablishedOrExternal, 1*time.Second)
	if err != nil {
		s.Fail(fmt.Sprintf("Failed establishing networking to the remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Network established to the remote cluster %q", remName))
	return nil
}

// ForOutgoingPeering waits until the status on the foreiglcusters resource states that the outgoing peering has been successfully
// established or the timeout expires.
func (w *Waiter) ForOutgoingPeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Activating outgoing peering to the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsOutgoingJoined, 1*time.Second)
	if err != nil {
		s.Fail(fmt.Sprintf("Failed activating outgoing peering to the remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Outgoing peering activated to the remote cluster %q", remName))
	return nil
}

// ForIncomingPeering waits until the status on the foreiglcusters resource states that the incoming peering has been successfully
// set to Yes or the timeout expires.
func (w *Waiter) ForIncomingPeering(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Activating incoming peering to the remote cluster %q", remName))
	err := fcutils.PollForEvent(ctx, w.CRClient, remoteClusterID, fcutils.IsIncomingPeeringYes, 1*time.Second)
	if err != nil {
		s.Fail(fmt.Sprintf("Failed activating outgoing peering to the remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Incoming peering activated to the remote cluster %q", remName))
	return nil
}

// ForNode waits until the node has been added to the cluster or the timeout expires.
func (w *Waiter) ForNode(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	remName := remoteClusterID.ClusterName
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for node to be created for the remote cluster %q", remName))

	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		nodes, err := getters.ListNodesByClusterID(ctx, w.CRClient, remoteClusterID)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		for i := range nodes.Items {
			if !utils.IsNodeReady(&nodes.Items[i]) {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for node to be created for remote cluster %q: %s", remName, output.PrettyErr(err)))
		return err
	}
	s.Success(fmt.Sprintf("Node created for remote cluster %q", remName))
	return nil
}

// ForOffloading waits until the status on the NamespaceOffloading resource states that the offloading has been successfully
// established or the timeout expires.
func (w *Waiter) ForOffloading(ctx context.Context, namespace string) error {
	s := w.Printer.StartSpinner(fmt.Sprintf("Waiting for offloading of namespace %q to complete", namespace))
	noClusterSelected := false
	var offload *offloadingv1alpha1.NamespaceOffloading
	err := wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true, func(ctx context.Context) (done bool, err error) {
		offload, err = getters.GetOffloadingByNamespace(ctx, w.CRClient, namespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		// Retry in case the observed generation does not match, as the status still needs to be updated.
		if offload.Status.ObservedGeneration != offload.GetGeneration() {
			return false, nil
		}

		someFailed := offload.Status.OffloadingPhase == offloadingv1alpha1.SomeFailedOffloadingPhaseType
		allFailed := offload.Status.OffloadingPhase == offloadingv1alpha1.AllFailedOffloadingPhaseType
		if someFailed || allFailed {
			return true, fmt.Errorf("the offloading is in %q state", offload.Status.OffloadingPhase)
		}

		ready := offload.Status.OffloadingPhase == offloadingv1alpha1.ReadyOffloadingPhaseType
		noClusterSelected = offload.Status.OffloadingPhase == offloadingv1alpha1.NoClusterSelectedOffloadingPhaseType

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
// successfully applied.
func (w *Waiter) ForConfiguration(ctx context.Context, conf *networkingv1alpha1.Configuration) error {
	s := w.Printer.StartSpinner("Waiting for configuration to be applied")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		ok, err := configuration.IsConfigurationStatusSet(ctx, w.CRClient, conf.Name, conf.Namespace)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return ok, nil
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
func (w *Waiter) ForGatewayServerStatusEndpoint(ctx context.Context, gwServer *networkingv1alpha1.GatewayServer) error {
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
func (w *Waiter) ForGatewayServerSecretRef(ctx context.Context, gwServer *networkingv1alpha1.GatewayServer) error {
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
func (w *Waiter) ForGatewayClientSecretRef(ctx context.Context, gwClient *networkingv1alpha1.GatewayClient) error {
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
	remoteCluster *discoveryv1alpha1.ClusterIdentity) (*networkingv1alpha1.Connection, error) {
	s := w.Printer.StartSpinner("Waiting for Connection to be created")
	var conn *networkingv1alpha1.Connection
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		remoteClusterIDSelector := labels.Set{consts.RemoteClusterID: remoteCluster.ClusterID}.AsSelector()
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
			return false, fmt.Errorf("more than one Connection resource found for remote cluster %q", remoteCluster.ClusterName)
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
func (w *Waiter) ForConnectionEstablished(ctx context.Context, conn *networkingv1alpha1.Connection) error {
	s := w.Printer.StartSpinner("Waiting for Connection status to be established")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		err = w.CRClient.Get(ctx, client.ObjectKeyFromObject(conn), conn)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}
		return conn.Status.Value == networkingv1alpha1.Connected, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for Connection status to be established: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Connection is established")
	return nil
}
