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
	"fmt"
	"reflect"
	"time"

	v1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	sharingv1alpha1 "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/pkg/vkMachinery"
	"github.com/liqotech/liqo/pkg/vkMachinery/forge"
)

const resourceOfferAnnotation = "liqo.io/resourceoffer"

// ResourceOfferReconciler reconciles a ResourceOffer object.
type ResourceOfferReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	eventsRecorder record.EventRecorder
	cluster        discoveryv1alpha1.ClusterIdentity

	liqoNamespace string

	virtualKubeletOpts *forge.VirtualKubeletOpts
	disableAutoAccept  bool

	resyncPeriod time.Duration
}

//+kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=sharing.liqo.io,resources=resourceoffers/finalizers,verbs=get;update;patch
//+kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/finalizers,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete;deletecollection
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;create;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResourceOfferReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	klog.V(4).Infof("reconciling ResourceOffer %v", req.NamespacedName)
	// get resource offer
	var resourceOffer sharingv1alpha1.ResourceOffer
	if err = r.Get(ctx, req.NamespacedName, &resourceOffer); err != nil {
		if kerrors.IsNotFound(err) {
			// reconcile was triggered by a delete request
			klog.Infof("ResourceRequest %v deleted", req.NamespacedName)
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
		// not managed error
		klog.Error(err)
		return ctrl.Result{}, err
	}
	originalResourceOffer := resourceOffer.DeepCopy()

	// we do that on ResourceOffer creation
	if metav1.GetControllerOf(&resourceOffer) == nil {
		if err = r.setControllerReference(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		if err = r.Client.Update(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		// we always return after a metadata or spec update to have a clean resource where to work
		return ctrl.Result{}, nil
	}

	result = ctrl.Result{RequeueAfter: r.resyncPeriod}

	// defer the status update function
	defer func() {
		if !reflect.
			DeepEqual(originalResourceOffer.ObjectMeta, resourceOffer.ObjectMeta) || !reflect.
			DeepEqual(originalResourceOffer.Spec, resourceOffer.Spec) {
			// create a copy of the status, as it will be overwritten by the update operation
			statusCopy := resourceOffer.Status.DeepCopy()
			// something changed in metadata (e.g. finalizers), or in the spec
			if newErr := r.Client.Update(ctx, &resourceOffer); newErr != nil {
				klog.Error(newErr)
				err = newErr
				return
			}
			resourceOffer.Status = *statusCopy
		}
		if newErr := r.Client.Status().Update(ctx, &resourceOffer); newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}()

	// filter resource offers and create a virtual-kubelet only for the good ones
	r.setResourceOfferPhase(&resourceOffer)

	// check the virtual kubelet deployment
	if err = r.checkVirtualKubeletDeployment(ctx, &resourceOffer); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// delete the ClusterRoleBinding if the VirtualKubelet Deployment is not up
	if resourceOffer.Status.VirtualKubeletStatus == sharingv1alpha1.VirtualKubeletStatusNone {
		if err = r.deleteClusterRoleBinding(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
	}

	deletingPhase := getDeleteVirtualKubeletPhase(&resourceOffer)
	switch deletingPhase {
	case kubeletDeletePhaseNodeDeleted:
		// delete virtual kubelet deployment
		if err = r.deleteVirtualKubeletDeployment(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		resourceOffer.Status.VirtualKubeletStatus = sharingv1alpha1.VirtualKubeletStatusNone
		return result, nil
	case kubeletDeletePhaseDrainingNode:
		// set virtual kubelet in deleting phase
		resourceOffer.Status.VirtualKubeletStatus = sharingv1alpha1.VirtualKubeletStatusDeleting
		return result, nil
	case kubeletDeletePhaseNone:
		// create the virtual kubelet deployment
		if err = r.createVirtualKubeletDeployment(ctx, &resourceOffer); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		return result, nil
	default:
		err = fmt.Errorf("unknown deleting phase %v", deletingPhase)
		klog.Error(err)
		return result, err
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResourceOfferReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// select replicated resources only
	p, err := predicate.LabelSelectorPredicate(reflection.ReplicatedResourcesLabelSelector())
	if err != nil {
		klog.Error(err)
		return err
	}

	// select virtual kubelet deployments only
	deployPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: vkMachinery.KubeletBaseLabels,
	})
	if err != nil {
		klog.Error(err)
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&sharingv1alpha1.ResourceOffer{}, builder.WithPredicates(p)).
		Watches(&source.Kind{Type: &v1.Deployment{}},
			getVirtualKubeletEventHandler(), builder.WithPredicates(deployPredicate)).
		Complete(r)
}

// getVirtualKubeletEventHandler creates and returns an event handle with the same behavior of the
// owner reference event handler, but using an annotation. This allows us to have a graceful deletion
// of the owned object, impossible using a standard owner reference, keeping the possibility to be
// triggered on children updates and to enforce their status.
func getVirtualKubeletEventHandler() handler.EventHandler {
	return &handler.Funcs{
		CreateFunc: func(ce event.CreateEvent, rli workqueue.RateLimitingInterface) {
			if req, err := getReconcileRequestFromObject(ce.Object); err == nil {
				rli.Add(req)
			}
		},
		UpdateFunc: func(ue event.UpdateEvent, rli workqueue.RateLimitingInterface) {
			if req, err := getReconcileRequestFromObject(ue.ObjectOld); err == nil {
				rli.Add(req)
			}
			if req, err := getReconcileRequestFromObject(ue.ObjectNew); err == nil {
				rli.Add(req)
			}
		},
		DeleteFunc: func(de event.DeleteEvent, rli workqueue.RateLimitingInterface) {
			if req, err := getReconcileRequestFromObject(de.Object); err == nil {
				rli.Add(req)
			}
		},
		GenericFunc: func(ge event.GenericEvent, rli workqueue.RateLimitingInterface) {
			if req, err := getReconcileRequestFromObject(ge.Object); err == nil {
				rli.Add(req)
			}
		},
	}
}

// getReconcileRequestFromObject reads an annotation in the object and returns the reconcile request
// to be enqueued for the reconciliation.
func getReconcileRequestFromObject(obj client.Object) (reconcile.Request, error) {
	resourceOfferName, ok := obj.GetAnnotations()[resourceOfferAnnotation]
	if !ok {
		return reconcile.Request{}, fmt.Errorf("%v annotation not found in object %v/%v",
			resourceOfferAnnotation, obj.GetNamespace(), obj.GetName())
	}

	return reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      resourceOfferName,
			Namespace: obj.GetNamespace(),
		},
	}, nil
}
