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

package quotacreatorcontroller

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqo-controller-manager/authentication"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// QuotaCreatorReconciler manage Quota lifecycle.
type QuotaCreatorReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	EventsRecorder record.EventRecorder

	DefaultLimitsEnforcement offloadingv1beta1.LimitsEnforcement
}

// NewQuotaCreatorReconciler returns a new QuotaCreatorReconciler.
func NewQuotaCreatorReconciler(
	cl client.Client,
	s *runtime.Scheme, er record.EventRecorder,
	defaultLimitsEnforcement offloadingv1beta1.LimitsEnforcement,
) *QuotaCreatorReconciler {
	return &QuotaCreatorReconciler{
		Client:         cl,
		Scheme:         s,
		EventsRecorder: er,

		DefaultLimitsEnforcement: defaultLimitsEnforcement,
	}
}

// cluster-role
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices,verbs=get;list;watch
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices/finalizers,verbs=update
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=quotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=quotas/finalizers,verbs=update

// Reconcile manage Quotas resources.
func (r *QuotaCreatorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	resourceSlice := &authv1beta1.ResourceSlice{}
	if err := r.Get(ctx, req.NamespacedName, resourceSlice); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get the ResourceSlice %q: %w", req.NamespacedName, err)
	}

	resourcesCond := authentication.GetCondition(resourceSlice, authv1beta1.ResourceSliceConditionTypeResources)
	resourcesAccepted := resourcesCond != nil && resourcesCond.Status == authv1beta1.ResourceSliceConditionAccepted
	if !resourcesAccepted {
		klog.V(3).Infof("ResourceSlice %s/%s resources not accepted yet", resourceSlice.Namespace, resourceSlice.Name)
		return ctrl.Result{}, nil
	}

	userName := authentication.CommonNameResourceSliceCSR(resourceSlice)

	quota := offloadingv1beta1.Quota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      userName,
			Namespace: resourceSlice.Namespace,
		},
	}
	_, err := resource.CreateOrUpdate(ctx, r.Client, &quota, func() error {
		quota.Spec.User = userName
		quota.Spec.LimitsEnforcement = r.DefaultLimitsEnforcement
		quota.Spec.Resources = resourceSlice.Status.Resources.DeepCopy()

		if hasToBeCordoned(resourceSlice) {
			quota.Spec.Cordoned = ptr.To(true)
		} else {
			quota.Spec.Cordoned = nil
		}

		return controllerutil.SetControllerReference(resourceSlice, &quota, r.Scheme)
	})
	if err != nil {
		klog.Errorf("Error while creating or updating Quota %s: %s", quota.Name, err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func hasToBeCordoned(resourceSlice *authv1beta1.ResourceSlice) bool {
	if resourceSlice.Annotations == nil {
		return false
	}
	isFalse := func(v string) bool {
		return v == "false" || v == "False" || v == "0"
	}

	v, ok := resourceSlice.Annotations[consts.CordonResourceAnnotation]
	sliceCordoned := ok && !isFalse(v)

	v, ok = resourceSlice.Annotations[consts.CordonTenantAnnotation]
	tenantCordoned := ok && !isFalse(v)

	return sliceCordoned || tenantCordoned
}

// SetupWithManager register the QuotaCreatorReconciler to the manager.
func (r *QuotaCreatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// generate the predicate to filter just the ResourceSlices created by the remote cluster checking crdReplicator labels
	remoteResSliceFilter, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named(consts.CtrlResourceSliceQuotaCreator).
		For(&authv1beta1.ResourceSlice{}, builder.WithPredicates(remoteResSliceFilter)).
		Owns(&offloadingv1beta1.Quota{}).
		Complete(r)
}
