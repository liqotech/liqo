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

package localrenwercontroller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/utils/events"
)

// LocalRenewerReconciler reconciles an Identity object.
type LocalRenewerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	LiqoNamespace  string
	LocalClusterID liqov1beta1.ClusterID
	recorder       record.EventRecorder
}

// NewLocalRenewerReconciler returns a new LocalRenewerReconciler.
func NewLocalRenewerReconciler(cl client.Client, s *runtime.Scheme,
	liqoNamespace string,
	localClusterID liqov1beta1.ClusterID,
	recorder record.EventRecorder) *LocalRenewerReconciler {
	return &LocalRenewerReconciler{
		Client:         cl,
		Scheme:         s,
		LiqoNamespace:  liqoNamespace,
		LocalClusterID: localClusterID,
		recorder:       recorder,
	}
}

//+kubebuilder:rbac:groups=authentication.liqo.io,resources=identities,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=identities/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=identities/finalizers,verbs=update
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile implements the logic to determine if an Identity should be renewed,
// and enforces the creation of a Renew object if needed. It also removes the
// current Renew object if the Identity does not need renewal anymore.
//
// The function first retrieves the Identity object and checks if it should be
// renewed using the shouldRenew function. Renewal can be triggered either by
// the presence of a "liqo.io/renew" annotation set to true, or by the certificate
// approaching its expiration time (2/3 of its lifetime).
//
// If the Identity does not need renewal, it removes the current Renew object
// if present and returns a requeue time calculated by the shouldRenew function.
//
// If the Identity needs renewal, the function creates a Renew object and returns
// a nil error. If an error occurs during the process, the function logs the
// error and returns it.
func (r *LocalRenewerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Get the Identity
	var identity authv1beta1.Identity
	if err := r.Get(ctx, req.NamespacedName, &identity); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	shouldRenew, requeueAfter, err := r.shouldRenew(ctx, &identity)
	if err != nil {
		klog.Errorf("Unable to check if Identity %q should renew: %s", req.NamespacedName, err)
		return ctrl.Result{}, err
	}

	if !shouldRenew {
		klog.V(4).Infof("Identity %q does not need renewal", req.NamespacedName)

		// Remove current Renew if present
		if err := r.removeCurrentRenew(ctx, &identity); err != nil {
			klog.Errorf("Unable to remove current Renew for Identity %q: %s", req.NamespacedName, err)
			return ctrl.Result{}, err
		}

		return ctrl.Result{
			RequeueAfter: requeueAfter,
		}, nil
	}

	if err := r.enforceRenew(ctx, &identity); err != nil {
		klog.Errorf("Unable to create Renew for Identity %q: %s", req.NamespacedName, err)
		events.EventWithOptions(r.recorder, &identity, fmt.Sprintf("Failed to create Renew: %s", err),
			&events.Option{EventType: events.Error, Reason: "RenewCreationFailed"})
		return ctrl.Result{}, err
	}

	klog.V(4).Infof("Created Renew for Identity %q", req.NamespacedName)
	events.Event(r.recorder, &identity, "Created Renew object for certificate renewal")

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LocalRenewerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlRenewLocal).
		Owns(&authv1beta1.Renew{}).
		For(&authv1beta1.Identity{}).
		Complete(r)
}

// shouldRenew determines whether an Identity object needs certificate renewal.
//
// The function first checks if the Identity has the renewal annotation set to true.
// If the annotation is present, it immediately triggers renewal regardless of certificate status.
//
// Otherwise, it retrieves the kubeconfig secret referenced by the Identity and checks the
// signed certificate within. The function calculates the certificate's lifetime
// and determines if a renewal is required based on the 2/3 life rule.
// If the certificate is not near expiration, it calculates the next check time
// as the remaining time until the 2/3 point plus a 10% buffer.
// If the certificate is near expiration, it checks if a Renew object already exists.
//
// Args:
//   - ctx: the context of the request
//   - identity: the Identity object to be checked
//
// Returns:
//   - renew: true if renewal is needed (either by annotation or certificate expiration)
//   - requeueIn: duration after which to requeue the reconciliation
//   - err: any error encountered during the process
func (r *LocalRenewerReconciler) shouldRenew(ctx context.Context, identity *authv1beta1.Identity) (renew bool, requeueIn time.Duration, err error) {
	// Check if the Identity has the renewal annotation
	if identity.Annotations != nil && (identity.Annotations[consts.RenewAnnotation] == "true" ||
		identity.Annotations[consts.RenewAnnotation] == "True") {
		return true, requeueIn, nil
	}

	// Get the kubeconfig secret referenced by the Identity
	if identity.Status.KubeconfigSecretRef == nil {
		return false, requeueIn, fmt.Errorf("identity %s/%s has no kubeconfig secret reference", identity.Namespace, identity.Name)
	}

	// Get the signed certificate from the kubeconfig
	signedCrt := identity.Spec.AuthParams.SignedCRT
	if len(signedCrt) == 0 {
		return false, requeueIn, fmt.Errorf("identity %s/%s has no signed certificate", identity.Namespace, identity.Name)
	}

	// Parse the certificate to get its expiration time
	block, _ := pem.Decode(signedCrt)
	if block == nil {
		return false, requeueIn, fmt.Errorf("failed to decode PEM block containing certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, requeueIn, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Calculate if we need to renew based on 2/3 life rule
	lifetime := cert.NotAfter.Sub(cert.NotBefore)
	twoThirdsPoint := cert.NotAfter.Add(-lifetime / 3)

	if time.Now().Before(twoThirdsPoint) {
		// Calculate requeue time as the remaining time until the 2/3 point of the certificate expiration time + 10%
		timeUntilTwoThirds := time.Until(twoThirdsPoint)
		requeueIn = timeUntilTwoThirds * 11 / 10

		klog.V(4).Infof("Certificate not ready for renewal, will check again in %v", requeueIn)
		return false, requeueIn, nil
	}

	// If certificate is not near expiration, check if Renew already exists
	var existingRenew authv1beta1.Renew
	err = r.Get(ctx, client.ObjectKey{
		Namespace: identity.Namespace,
		Name:      identity.Name,
	}, &existingRenew)
	if err == nil {
		// Renew already exists, skip
		return false, requeueIn, nil
	}

	return true, requeueIn, nil // No existing Renew, proceed with creation
}

// enforceRenew enforces the creation of a Renew object for the given Identity.
//
// The function creates a Renew object with the same name and namespace as the given Identity.
// The Renew object is filled with the public key of the local cluster and a CSR for the remote cluster.
// The CSR is generated based on the IdentityType of the given Identity.
// If the IdentityType is ControlPlaneIdentityType, the CSR is generated using the GenerateCSRForControlPlane function.
// If the IdentityType is ResourceSliceIdentityType, the CSR is generated using the GenerateCSRForResourceSlice function.
// The function sets the owner reference of the Renew object to the given Identity.
// The function returns an error if it fails to get the cluster keys or generate the CSR.
func (r *LocalRenewerReconciler) enforceRenew(ctx context.Context, identity *authv1beta1.Identity) error {
	// Create or update the Renew object
	renew := &authv1beta1.Renew{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identity.Name,
			Namespace: identity.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, renew, func() error {
		// Set replication labels
		if renew.Labels == nil {
			renew.Labels = make(map[string]string)
		}
		renew.Labels[consts.ReplicationRequestedLabel] = consts.ReplicationRequestedLabelValue
		renew.Labels[consts.ReplicationDestinationLabel] = string(identity.Spec.ClusterID)
		renew.Labels[consts.RemoteClusterID] = string(identity.Spec.ClusterID)

		// Set owner reference
		if err := controllerutil.SetControllerReference(identity, renew, r.Scheme); err != nil {
			return err
		}

		// Copy fields from Identity to Renew
		renew.Spec.ConsumerClusterID = r.LocalClusterID
		renew.Spec.IdentityType = identity.Spec.Type

		// If this is a ResourceSlice identity, set the reference
		if identity.Spec.Type == authv1beta1.ResourceSliceIdentityType {
			// Find the ResourceSlice owner reference
			for _, ownerRef := range identity.GetOwnerReferences() {
				if ownerRef.Kind == authv1beta1.ResourceSliceKind {
					renew.Spec.ResourceSliceRef = &corev1.LocalObjectReference{
						Name: ownerRef.Name,
					}
					break
				}
			}
		}

		// Get public and private keys of the local cluster.
		privateKey, publicKey, err := authentication.GetClusterKeys(ctx, r.Client, r.LiqoNamespace)
		if err != nil {
			return fmt.Errorf("unable to get cluster keys: %w", err)
		}

		renew.Spec.PublicKey = publicKey

		switch identity.Spec.Type {
		case authv1beta1.ControlPlaneIdentityType:
			// Generate a CSR for the remote cluster.
			CSR, err := authentication.GenerateCSRForControlPlane(privateKey, identity.Spec.ClusterID)
			if err != nil {
				return fmt.Errorf("unable to generate CSR: %w", err)
			}
			renew.Spec.CSR = CSR
		case authv1beta1.ResourceSliceIdentityType:
			var resourceSlice authv1beta1.ResourceSlice
			if err := r.Get(ctx, client.ObjectKey{
				Namespace: identity.Namespace,
				Name:      renew.Spec.ResourceSliceRef.Name,
			}, &resourceSlice); err != nil {
				return fmt.Errorf("unable to get ResourceSlice: %w", err)
			}

			// Generate a CSR for the remote cluster.
			CSR, err := authentication.GenerateCSRForResourceSlice(privateKey, &resourceSlice)
			if err != nil {
				return fmt.Errorf("unable to generate CSR: %w", err)
			}
			renew.Spec.CSR = CSR
		}

		return nil
	})

	return err
}

// removeCurrentRenew removes the current Renew object for the given Identity.
//
// The function deletes the Renew object with the same name and namespace as the given Identity.
// If the deletion fails, it returns an error.
func (r *LocalRenewerReconciler) removeCurrentRenew(ctx context.Context, identity *authv1beta1.Identity) error {
	return client.IgnoreNotFound(r.Delete(ctx, &authv1beta1.Renew{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identity.Name,
			Namespace: identity.Namespace,
		},
	}))
}
