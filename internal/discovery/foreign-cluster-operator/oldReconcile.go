package foreignclusteroperator

import (
	"context"
	goerrors "errors"
	"fmt"
	"reflect"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/discovery"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// nolint:gocyclo // (aleoli): Suppressing for now, it will be deleted.
func (r *ForeignClusterReconciler) oldReconcile(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster) (ctrl.Result, error) {
	oldStatus := foreignCluster.Status.DeepCopy()

	// check for NetworkConfigs
	if err := r.checkNetwork(ctx, foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// check for TunnelEndpoints
	if err := r.checkTEP(ctx, foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// TODO: to be removed
	// check if linked advertisement exists
	if foreigncluster.IsOutgoingEnabled(foreignCluster) {
		requireUpdate := false
		requireSpecUpdate := false
		tmp, err := r.advertisementClient.Resource("advertisements").
			Get(fmt.Sprintf("advertisement-%s", foreignCluster.Spec.ClusterIdentity.ClusterID), &metav1.GetOptions{})
		if errors.IsNotFound(err) && foreignCluster.Status.Outgoing.AvailableIdentity {
			foreignCluster.Status.Outgoing.Advertisement = nil
			foreignCluster.Status.Outgoing.AvailableIdentity = false
			foreignCluster.Status.Outgoing.IdentityRef = nil
			foreignCluster.Status.Outgoing.AdvertisementStatus = ""
			foreignCluster.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseNone
			foreignCluster.Status.Outgoing.RemotePeeringRequestName = ""
			foreignCluster.Spec.Join = false
			requireUpdate = true
			requireSpecUpdate = true
		} else if err == nil {
			// check if kubeconfig secret exists
			adv, ok := tmp.(*advtypes.Advertisement)
			if !ok {
				err = goerrors.New("retrieved object is not an Advertisement")
				klog.Error(err)
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: r.RequeueAfter,
				}, err
			}
			if adv.Spec.KubeConfigRef.Name != "" && adv.Spec.KubeConfigRef.Namespace != "" {
				_, err = r.crdClient.Client().CoreV1().Secrets(
					adv.Spec.KubeConfigRef.Namespace).Get(
					context.TODO(), adv.Spec.KubeConfigRef.Name, metav1.GetOptions{})
				available := err == nil
				if foreignCluster.Status.Outgoing.AvailableIdentity != available {
					foreignCluster.Status.Outgoing.AvailableIdentity = available
					if available {
						foreignCluster.Status.Outgoing.IdentityRef = &apiv1.ObjectReference{
							Kind:       "Secret",
							Namespace:  adv.Spec.KubeConfigRef.Namespace,
							Name:       adv.Spec.KubeConfigRef.Name,
							APIVersion: "v1",
						}
					}
					requireUpdate = true
				}
			}

			// update advertisement status
			status := adv.Status.AdvertisementStatus
			if status != foreignCluster.Status.Outgoing.AdvertisementStatus {
				foreignCluster.Status.Outgoing.AdvertisementStatus = status
				requireUpdate = true
			}
		}

		if requireSpecUpdate {
			status := foreignCluster.Status.DeepCopy()
			if foreignCluster, err = r.update(foreignCluster); err != nil {
				klog.Error(err)
				return ctrl.Result{}, err
			}
			foreignCluster.Status = *status
		}

		if requireUpdate {
			return r.updateStatus(ctx, foreignCluster)
		}
	}

	// TODO: to be removed
	// check if linked peeringRequest exists
	if foreignCluster.Status.Incoming.PeeringRequest != nil {
		requireUpdate := false
		tmp, err := r.crdClient.Resource("peeringrequests").Get(foreignCluster.Status.Incoming.PeeringRequest.Name, &metav1.GetOptions{})
		if errors.IsNotFound(err) {
			foreignCluster.Status.Incoming.PeeringRequest = nil
			foreignCluster.Status.Incoming.AvailableIdentity = false
			foreignCluster.Status.Incoming.IdentityRef = nil
			foreignCluster.Status.Incoming.AdvertisementStatus = ""
			foreignCluster.Status.Incoming.PeeringPhase = discoveryv1alpha1.PeeringPhaseNone
			requireUpdate = true
		} else if err == nil {
			pr, ok := tmp.(*discoveryv1alpha1.PeeringRequest)
			if !ok {
				err = goerrors.New("retrieved object is not a PeeringRequest")
				klog.Error(err)
				return ctrl.Result{
					Requeue:      true,
					RequeueAfter: r.RequeueAfter,
				}, err
			}

			if !foreigncluster.IsIncomingJoined(foreignCluster) {
				// PeeringRequest exists, set flag to true
				foreignCluster.Status.Incoming.PeeringPhase = discoveryv1alpha1.PeeringPhaseEstablished
				requireUpdate = true
			}

			// check if kubeconfig secret exists
			if pr.Spec.KubeConfigRef != nil && pr.Spec.KubeConfigRef.Name != "" && pr.Spec.KubeConfigRef.Namespace != "" {
				_, err = r.crdClient.Client().CoreV1().Secrets(
					pr.Spec.KubeConfigRef.Namespace).Get(context.TODO(), pr.Spec.KubeConfigRef.Name, metav1.GetOptions{})
				available := err == nil
				if foreignCluster.Status.Incoming.AvailableIdentity != available {
					foreignCluster.Status.Incoming.AvailableIdentity = available
					if available {
						foreignCluster.Status.Incoming.IdentityRef = pr.Spec.KubeConfigRef
					}
					requireUpdate = true
				}
			}

			// update advertisement status
			status := pr.Status.AdvertisementStatus
			if status != foreignCluster.Status.Incoming.AdvertisementStatus {
				foreignCluster.Status.Incoming.AdvertisementStatus = status
				requireUpdate = true
			}
		}

		if requireUpdate {
			return r.updateStatus(ctx, foreignCluster)
		}
	}

	// if it has been discovered thanks to incoming peeringRequest and it has no active connections, delete it
	isIncomingDiscovery := foreignCluster.Spec.DiscoveryType == discovery.IncomingPeeringDiscovery
	isNotDeleting := foreignCluster.DeletionTimestamp.IsZero()
	hasPeering := foreigncluster.IsIncomingEnabled(foreignCluster) || foreigncluster.IsOutgoingEnabled(foreignCluster)
	if isIncomingDiscovery && isNotDeleting && !hasPeering {
		klog.Infof("%s deleted, discovery type %s has no active connections", foreignCluster.Name, foreignCluster.Spec.DiscoveryType)
		if err := r.deleteForeignCluster(ctx, foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
		return ctrl.Result{}, nil
	}

	foreignDiscoveryClient, err := r.getRemoteClient(foreignCluster, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	} else if foreignDiscoveryClient == nil {
		return r.updateStatus(ctx, foreignCluster)
	}

	if foreignDiscoveryClient != nil && foreignCluster.Spec.Join && !foreigncluster.IsOutgoingEnabled(foreignCluster) {
		fc, err := r.Peer(foreignCluster, foreignDiscoveryClient)
		if err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		return r.updateStatus(ctx, fc)
	}

	if foreignDiscoveryClient != nil && (!foreignCluster.Spec.Join || !foreignCluster.DeletionTimestamp.
		IsZero()) && foreigncluster.IsOutgoingEnabled(foreignCluster) {
		fc, err := r.Unpeer(foreignCluster, foreignDiscoveryClient)
		if err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		return r.updateStatus(ctx, fc)
	}

	if !foreignCluster.Spec.Join && !foreigncluster.
		IsOutgoingEnabled(foreignCluster) && slice.
		ContainsString(foreignCluster.Finalizers, FinalizerString, nil) {
		if result, err := r.removeFinalizer(ctx, foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		} else if result != controllerutil.OperationResultNone {
			return ctrl.Result{}, nil
		}
	}

	// check if peering request really exists on foreign cluster
	if foreignDiscoveryClient != nil && foreignCluster.Spec.Join && foreigncluster.IsOutgoingEnabled(foreignCluster) {
		_, err = r.checkJoined(foreignCluster, foreignDiscoveryClient)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
	}

	if !reflect.DeepEqual(&foreignCluster.Status, oldStatus) {
		return r.updateStatus(ctx, foreignCluster)
	}

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.RequeueAfter,
	}, nil
}
