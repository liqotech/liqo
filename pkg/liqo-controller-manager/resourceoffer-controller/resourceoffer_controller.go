/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resourceoffercontroller

import (
	"context"
	"sync"
	"time"

	v1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/pkg/clusterid"
)

// ResourceOfferReconciler reconciles a ResourceOffer object.
type ResourceOfferReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	eventsRecorder record.EventRecorder
	clusterID      clusterid.ClusterID

	virtualKubeletImage     string
	initVirtualKubeletImage string

	resyncPeriod       time.Duration
	configuration      *configv1alpha1.ClusterConfig
	configurationMutex sync.RWMutex
}

//+kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceOfferReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("reconciling ResourceOffer %v", req.NamespacedName)
	// get resource offer
	var resourceOffer sharingv1alpha1.ResourceOffer
	if err := r.Get(ctx, req.NamespacedName, &resourceOffer); err != nil {
		if kerrors.IsNotFound(err) {
			// reconcile was triggered by a delete request
			klog.Infof("ResourceRequest %v deleted", req.NamespacedName)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		// not managed error
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// we do that on ResourceOffer creation
	if result, err := r.setOwnerReference(ctx, &resourceOffer); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if result != controllerutil.OperationResultNone {
		return ctrl.Result{}, nil
	}

	// filter resource offers and create a virtual-kubelet only for the good ones
	if result, err := r.setResourceOfferPhase(ctx, &resourceOffer); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if result != controllerutil.OperationResultNone {
		return ctrl.Result{}, nil
	}

	// check the virtual kubelet deployment
	if result, err := r.checkVirtualKubeletDeployment(ctx, &resourceOffer); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if result != controllerutil.OperationResultNone {
		return ctrl.Result{}, nil
	}

	// delete the ClusterRoleBinding if the VirtualKubelet Deployment is not up
	if resourceOffer.Status.VirtualKubeletStatus == sharingv1alpha1.VirtualKubeletStatusNone {
		if err := r.deleteClusterRoleBinding(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
	}

	if !isAccepted(&resourceOffer) || !resourceOffer.DeletionTimestamp.IsZero() {
		// delete virtual kubelet deployment
		if result, err := r.deleteVirtualKubeletDeployment(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		} else if result != controllerutil.OperationResultNone {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: r.resyncPeriod}, nil
	}

	// create the virtual kubelet deployment
	if result, err := r.createVirtualKubeletDeployment(ctx, &resourceOffer); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if result != controllerutil.OperationResultNone {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: r.resyncPeriod}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceOfferReconciler) SetupWithManager(mgr ctrl.Manager) error {
	selector, err := metav1.LabelSelectorAsSelector(&crdreplicator.ReplicatedResourcesLabelSelector)
	if err != nil {
		klog.Error(err)
		return err
	}

	p := predicate.NewPredicateFuncs(func(object client.Object) bool {
		matches := selector.Matches(labels.Set(object.GetLabels()))
		_, isResourceOffer := object.(*sharingv1alpha1.ResourceOffer)
		return matches || !isResourceOffer
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&sharingv1alpha1.ResourceOffer{}).
		Owns(&v1.Deployment{}).
		WithEventFilter(p).
		Complete(r)
}
