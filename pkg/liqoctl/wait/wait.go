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

	"github.com/pterm/pterm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1alpha1 "github.com/liqotech/liqo/apis/authentication/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	noncesigner "github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication/noncesigner-controller"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
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

// ForResourceSlice waits until the ResourceSlice has been accepted or the timeout expires.
func (w *Waiter) ForResourceSlice(ctx context.Context, resourceSlice *authv1alpha1.ResourceSlice) error {
	s := w.Printer.StartSpinner("Waiting for ResourceSlice to be accepted")

	nsName := client.ObjectKeyFromObject(resourceSlice)
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		if err := w.CRClient.Get(ctx, nsName, resourceSlice); err != nil {
			return false, client.IgnoreNotFound(err)
		}

		authCondition := authentication.GetCondition(resourceSlice, authv1alpha1.ResourceSliceConditionTypeAuthentication)
		resourcesCondition := authentication.GetCondition(resourceSlice, authv1alpha1.ResourceSliceConditionTypeResources)
		if authCondition != nil && authCondition.Status == authv1alpha1.ResourceSliceConditionAccepted &&
			resourcesCondition != nil && resourcesCondition.Status == authv1alpha1.ResourceSliceConditionAccepted {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for ResourceSlice to be accepted: %s", output.PrettyErr(err)))
		return err
	}

	s.Success("ResourceSlice accepted")
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

// ForSignedNonce waits until the signed nonce secret has been signed and returns the signature.
func (w *Waiter) ForSignedNonce(ctx context.Context, clusterID string, silent bool) ([]byte, error) {
	var nonceSecret *corev1.Secret
	var signedNonce []byte
	var s *pterm.SpinnerPrinter

	if !silent {
		s = w.Printer.StartSpinner("Waiting for nonce to be signed")
	}

	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		nonceSecret, err = noncesigner.GetSignedNonceSecret(ctx, w.CRClient, clusterID)
		if client.IgnoreNotFound(err) != nil {
			return false, err
		}
		if signedNonce, err = noncesigner.GetSignedNonceFromSecret(nonceSecret); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		if !silent {
			s.Fail(fmt.Sprintf("Failed waiting for nonce to be signed: %s", output.PrettyErr(err)))
		}
		return nil, err
	}

	if !silent {
		s.Success("Nonce is signed")
	}

	return signedNonce, nil
}

// ForNonce waits until the secret containing the nonce has been created or the timeout expires.
func (w *Waiter) ForNonce(ctx context.Context, remoteClusterID *discoveryv1alpha1.ClusterIdentity) error {
	s := w.Printer.StartSpinner("Waiting for nonce to be created")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		secret, err := getters.GetNonceByClusterID(ctx, w.CRClient, remoteClusterID.ClusterID)
		if err != nil {
			return false, client.IgnoreNotFound(err)
		}

		_, ok := secret.Data[consts.NonceSecretField]
		return ok, nil
	})
	if err != nil {
		s.Fail(fmt.Sprintf("Failed waiting for nonce to be created: %s", output.PrettyErr(err)))
		return err
	}
	s.Success("Nonce created successfully")
	return nil
}

// ForTenantStatus waits until the tenant status has been updated or the timeout expires.
func (w *Waiter) ForTenantStatus(ctx context.Context, remoteClusterID string) error {
	s := w.Printer.StartSpinner("Waiting for tenant status to be filled")
	err := wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (done bool, err error) {
		tenant, err := getters.GetTenantByClusterID(ctx, w.CRClient, remoteClusterID)
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
