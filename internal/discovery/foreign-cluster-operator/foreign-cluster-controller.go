/*


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

package foreignclusteroperator

import (
	"context"
	goerrors "errors"
	"fmt"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/internal/discovery"
	"github.com/liqotech/liqo/internal/discovery/utils"
	"github.com/liqotech/liqo/pkg/auth"
	"github.com/liqotech/liqo/pkg/clusterid"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
	crdclient "github.com/liqotech/liqo/pkg/crdClient"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/kubeconfig"
	peeringRoles "github.com/liqotech/liqo/pkg/peering-roles"
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
)

// FinalizerString is added as finalizer for peered ForeignClusters.
const FinalizerString = "foreigncluster.discovery.liqo.io/peered"

// ForeignClusterReconciler reconciles a ForeignCluster object.
type ForeignClusterReconciler struct {
	Scheme *runtime.Scheme

	Namespace           string
	crdClient           *crdclient.CRDClient
	advertisementClient *crdclient.CRDClient
	networkClient       *crdclient.CRDClient
	clusterID           clusterid.ClusterID
	RequeueAfter        time.Duration

	namespaceManager tenantcontrolnamespace.TenantControlNamespaceManager
	identityManager  identitymanager.IdentityManager
	useNewAuth       bool

	peeringPermission peeringRoles.PeeringPermission

	ConfigProvider     discovery.ConfigProvider
	AuthConfigProvider auth.ConfigProvider

	// testing
	ForeignConfig *rest.Config
}

// clusterRole
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=searchdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=*
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=*
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups=net.liqo.io,resources=tunnelendpoints/status,verbs=get;watch;update
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=sharing.liqo.io,resources=advertisements/status,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;create
// tenant control namespace management
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;create;delete;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=create;deletecollection
// role
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=services,verbs=get
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=configmaps,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=serviceaccounts,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=roles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=rolebindings,verbs=get;create

// Reconcile reconciles ForeignCluster resources.
// nolint:gocyclo // (palexster): Suppressing for now, it will part of a major refactoring before v0.3
func (r *ForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)

	tmp, err := r.crdClient.Resource("foreignclusters").Get(req.Name, &metav1.GetOptions{})
	if err != nil {
		return ctrl.Result{}, nil
	}
	fc, ok := tmp.(*discoveryv1alpha1.ForeignCluster)
	if !ok {
		klog.Error("created object is not a ForeignCluster")
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, goerrors.New("created object is not a ForeignCluster")
	}

	requireUpdate := false

	if r.needsClusterIdentityDefaulting(fc) {
		// this ForeignCluster has not all the required fields, probably it has been added manually, so default to exposed values
		if err = r.clusterIdentityDefaulting(fc); err != nil {
			klog.Error(err)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		// the resource has been updated, no need to requeue
		return ctrl.Result{}, nil
	}

	// set trust property
	// This will only be executed in the ForeignCluster CR has been added in a manual way,
	// if it was discovered this field is set by the discovery process.
	// We can consider to move it in a mutating webhook
	if fc.Spec.TrustMode == discoveryPkg.TrustModeUnknown || fc.Spec.TrustMode == "" {
		trust, err := fc.CheckTrusted()
		if err != nil {
			klog.Error(err)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		if trust {
			fc.Spec.TrustMode = discoveryPkg.TrustModeTrusted
		} else {
			fc.Spec.TrustMode = discoveryPkg.TrustModeUntrusted
		}
		// set join flag
		// if it was discovery with WAN discovery, this value is overwritten by SearchDomain value
		isWan := fc.Spec.DiscoveryType == discoveryPkg.WanDiscovery
		isIncoming := fc.Spec.DiscoveryType == discoveryPkg.IncomingPeeringDiscovery
		isManual := fc.Spec.DiscoveryType == discoveryPkg.ManualDiscovery
		if !isWan && !isIncoming && !isManual {
			joinTrusted := r.getAutoJoin(fc) && fc.Spec.TrustMode == discoveryPkg.TrustModeTrusted
			joinUntrusted := r.getAutoJoinUntrusted(fc) && fc.Spec.TrustMode == discoveryPkg.TrustModeUntrusted
			fc.Spec.Join = joinTrusted || joinUntrusted
		}

		requireUpdate = true
	}

	// if it has no discovery type label, add it
	if fc.ObjectMeta.Labels == nil {
		fc.ObjectMeta.Labels = map[string]string{}
	}
	noLabel := fc.ObjectMeta.Labels[discoveryPkg.DiscoveryTypeLabel] == ""
	differentLabel := fc.ObjectMeta.Labels[discoveryPkg.DiscoveryTypeLabel] != string(fc.Spec.DiscoveryType)
	if noLabel || differentLabel {
		fc.ObjectMeta.Labels[discoveryPkg.DiscoveryTypeLabel] = string(fc.Spec.DiscoveryType)
		requireUpdate = true
	}
	// set cluster-id label to easy retrieve ForeignClusters by ClusterId,
	// if it is added manually, the name maybe not coincide with ClusterId
	if fc.ObjectMeta.Labels[discoveryPkg.ClusterIDLabel] == "" {
		fc.ObjectMeta.Labels[discoveryPkg.ClusterIDLabel] = fc.Spec.ClusterIdentity.ClusterID
		requireUpdate = true
	}

	if r.useNewAuth {
		// create the local TenantControlNamespace if it has not been created
		if update, err := r.createLocalTenantControlNamespace(fc); err != nil {
			klog.Error(err)
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		} else if update {
			requireUpdate = true
		}
	}

	// check for NetworkConfigs
	err = r.checkNetwork(fc, &requireUpdate)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}

	// check for TunnelEndpoints
	err = r.checkTEP(fc, &requireUpdate)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	}

	// check if linked advertisement exists
	if fc.Status.Outgoing.Advertisement != nil {
		tmp, err = r.advertisementClient.Resource("advertisements").Get(fc.Status.Outgoing.Advertisement.Name, &metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fc.Status.Outgoing.Advertisement = nil
			fc.Status.Outgoing.AvailableIdentity = false
			fc.Status.Outgoing.IdentityRef = nil
			fc.Status.Outgoing.AdvertisementStatus = ""
			fc.Status.Outgoing.Joined = false
			fc.Status.Outgoing.RemotePeeringRequestName = ""
			fc.Spec.Join = false
			requireUpdate = true
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
				if fc.Status.Outgoing.AvailableIdentity != available {
					fc.Status.Outgoing.AvailableIdentity = available
					if available {
						fc.Status.Outgoing.IdentityRef = &apiv1.ObjectReference{
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
			if status != fc.Status.Outgoing.AdvertisementStatus {
				fc.Status.Outgoing.AdvertisementStatus = status
				requireUpdate = true
			}
		}
	}

	// check if linked peeringRequest exists
	if fc.Status.Incoming.PeeringRequest != nil {
		tmp, err = r.crdClient.Resource("peeringrequests").Get(fc.Status.Incoming.PeeringRequest.Name, &metav1.GetOptions{})
		if errors.IsNotFound(err) {
			fc.Status.Incoming.PeeringRequest = nil
			fc.Status.Incoming.AvailableIdentity = false
			fc.Status.Incoming.IdentityRef = nil
			fc.Status.Incoming.AdvertisementStatus = ""
			fc.Status.Incoming.Joined = false
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

			if !fc.Status.Incoming.Joined {
				// PeeringRequest exists, set flag to true
				fc.Status.Incoming.Joined = true
				requireUpdate = true
			}

			// check if kubeconfig secret exists
			if pr.Spec.KubeConfigRef != nil && pr.Spec.KubeConfigRef.Name != "" && pr.Spec.KubeConfigRef.Namespace != "" {
				_, err = r.crdClient.Client().CoreV1().Secrets(
					pr.Spec.KubeConfigRef.Namespace).Get(context.TODO(), pr.Spec.KubeConfigRef.Name, metav1.GetOptions{})
				available := err == nil
				if fc.Status.Incoming.AvailableIdentity != available {
					fc.Status.Incoming.AvailableIdentity = available
					if available {
						fc.Status.Incoming.IdentityRef = pr.Spec.KubeConfigRef
					}
					requireUpdate = true
				}
			}

			// update advertisement status
			status := pr.Status.AdvertisementStatus
			if status != fc.Status.Incoming.AdvertisementStatus {
				fc.Status.Incoming.AdvertisementStatus = status
				requireUpdate = true
			}
		}
	}

	// if it has been discovered thanks to incoming peeringRequest and it has no active connections, delete it
	isIncomingDiscovery := fc.Spec.DiscoveryType == discoveryPkg.IncomingPeeringDiscovery
	isNotDeleting := fc.DeletionTimestamp.IsZero()
	hasPeering := fc.Status.Incoming.PeeringRequest == nil && fc.Status.Outgoing.Advertisement == nil
	if isIncomingDiscovery && isNotDeleting && hasPeering {
		err = r.crdClient.Resource("foreignclusters").Delete(fc.Name, &metav1.DeleteOptions{})
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		klog.Infof("%s deleted, discovery type %s has no active connections", fc.Name, fc.Spec.DiscoveryType)
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", fc.Name)
		return ctrl.Result{}, nil
	}

	// TODO: it's probably better to refactor it when we will remove the old authentication, we will need no more a remote client
	foreignDiscoveryClient, err := r.getRemoteClient(fc, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		klog.Error(err)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	} else if foreignDiscoveryClient == nil || r.useNewAuth {
		requireUpdate = true
	}

	if update, err := r.fetchRemoteTenantNamespace(fc); err != nil {
		klog.Error(err)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, err
	} else if update {
		requireUpdate = true
	}
	remoteNamespace := ""
	if r.useNewAuth {
		remoteNamespace = fc.Status.TenantControlNamespace.Remote
	}

	// if join is required (both automatically or by user) and status is not set to joined
	// create new peering request
	if (foreignDiscoveryClient != nil || remoteNamespace != "") && fc.Spec.Join && !fc.Status.Outgoing.Joined {
		fc, err = r.Peer(fc, foreignDiscoveryClient)
		if err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		requireUpdate = true
	}

	// if join is no more required and status is set to joined
	// or if this foreign cluster is being deleted
	// delete peering request
	if (foreignDiscoveryClient != nil || remoteNamespace != "") && (!fc.Spec.Join || !fc.DeletionTimestamp.IsZero()) && fc.Status.Outgoing.Joined {
		fc, err = r.Unpeer(fc, foreignDiscoveryClient)
		if err != nil {
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		requireUpdate = true
	}

	if !fc.Spec.Join && !fc.Status.Outgoing.Joined && slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
		fc.Finalizers = slice.RemoveString(fc.Finalizers, FinalizerString, nil)
		requireUpdate = true
	}

	if requireUpdate {
		_, err = r.update(fc)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", fc.Name)
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: r.RequeueAfter,
		}, nil
	}

	// check if peering request really exists on foreign cluster
	if (foreignDiscoveryClient != nil || remoteNamespace != "") && fc.Spec.Join && fc.Status.Outgoing.Joined {
		_, err = r.checkJoined(fc, foreignDiscoveryClient)
		if err != nil {
			klog.Error(err, err.Error())
			return ctrl.Result{
				Requeue:      true,
				RequeueAfter: r.RequeueAfter,
			}, err
		}
	}

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", fc.Name)
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: r.RequeueAfter,
	}, nil
}

func (r *ForeignClusterReconciler) update(fc *discoveryv1alpha1.ForeignCluster) (*discoveryv1alpha1.ForeignCluster, error) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if tmp, err := r.crdClient.Resource("foreignclusters").Update(fc.Name, fc, &metav1.UpdateOptions{}); err == nil {
			var ok bool
			fc, ok = tmp.(*discoveryv1alpha1.ForeignCluster)
			if !ok {
				err = goerrors.New("this object is not a ForeignCluster")
				klog.Error(err, tmp)
				return err
			}
			return nil
		} else if !errors.IsConflict(err) {
			return err
		}
		tmp, err := r.crdClient.Resource("foreignclusters").Get(fc.Name, &metav1.GetOptions{})
		if err != nil {
			klog.Error(err)
			return err
		}
		fc2, ok := tmp.(*discoveryv1alpha1.ForeignCluster)
		if !ok {
			err = goerrors.New("this object is not a ForeignCluster")
			klog.Error(err, tmp)
			return err
		}
		fc.ResourceVersion = fc2.ResourceVersion
		fc.Generation = fc2.Generation
		return err
	})
	return fc, err
}

// Peer creates the peering with a remote cluster.
func (r *ForeignClusterReconciler) Peer(
	fc *discoveryv1alpha1.ForeignCluster,
	foreignDiscoveryClient *crdclient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {

	if r.useNewAuth {
		return r.peerNamespaced(fc)
	}

	// create PeeringRequest
	klog.Infof("[%v] Creating PeeringRequest", fc.Spec.ClusterIdentity.ClusterID)
	pr, err := r.createPeeringRequestIfNotExists(fc.Name, fc, foreignDiscoveryClient)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if pr != nil {
		fc.Status.Outgoing.Joined = true
		fc.Status.Outgoing.RemotePeeringRequestName = pr.Name
		// add finalizer
		if !slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
			fc.Finalizers = append(fc.Finalizers, FinalizerString)
		}
	}
	return fc, nil
}

// peerNamespaced enables the peering creating the resources in the correct TenantControlNamespace.
func (r *ForeignClusterReconciler) peerNamespaced(foreignCluster *discoveryv1alpha1.ForeignCluster) (*discoveryv1alpha1.ForeignCluster, error) {
	// create ResourceRequest
	klog.Infof("[%v] Creating ResourceRequest", foreignCluster.Spec.ClusterIdentity.ClusterID)
	resourceRequest, err := r.createResourceRequest(foreignCluster)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	if resourceRequest != nil {
		foreignCluster.Status.Outgoing.Joined = true
	}
	return foreignCluster, nil
}

// Unpeer removes the peering with a remote cluster.
func (r *ForeignClusterReconciler) Unpeer(fc *discoveryv1alpha1.ForeignCluster,
	foreignDiscoveryClient *crdclient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {

	if r.useNewAuth {
		return r.unpeerNamespaced(fc)
	}

	// peering request has to be removed
	klog.Infof("[%v] Deleting PeeringRequest", fc.Spec.ClusterIdentity.ClusterID)
	err := r.deletePeeringRequest(foreignDiscoveryClient, fc)
	if err != nil && !errors.IsNotFound(err) {
		klog.Error(err)
		return nil, err
	}
	// local advertisement has to be removed
	err = r.deleteAdvertisement(fc)
	if err != nil && !errors.IsNotFound(err) {
		klog.Error(err)
		return nil, err
	}
	fc.Status.Outgoing.Joined = false
	fc.Status.Outgoing.RemotePeeringRequestName = ""
	if slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
		fc.Finalizers = slice.RemoveString(fc.Finalizers, FinalizerString, nil)
	}
	return fc, nil
}

// unpeerNamespaced disables the peering deleting the resources in the correct TenantControlNamespace.
func (r *ForeignClusterReconciler) unpeerNamespaced(foreignCluster *discoveryv1alpha1.ForeignCluster) (*discoveryv1alpha1.ForeignCluster, error) {
	// resource request has to be removed
	klog.Infof("[%v] Deleting ResourceRequest", foreignCluster.Spec.ClusterIdentity.ClusterID)
	err := r.deleteResourceRequest(foreignCluster)
	if err != nil && !errors.IsNotFound(err) {
		klog.Error(err)
		return nil, err
	}

	foreignCluster.Status.Outgoing.Joined = false
	return foreignCluster, nil
}

// SetupWithManager assigns the operator to a manager.
func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}).
		Owns(&advtypes.Advertisement{}).
		Owns(&discoveryv1alpha1.PeeringRequest{}).
		Complete(r)
}

// checkJoined checks the outgoing join status of a cluster.
func (r *ForeignClusterReconciler) checkJoined(fc *discoveryv1alpha1.ForeignCluster,
	foreignDiscoveryClient *crdclient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {

	if r.useNewAuth {
		_, err := r.crdClient.Resource("resourcerequests").Namespace(
			fc.Status.TenantControlNamespace.Local).Get(r.clusterID.GetClusterID(), &metav1.GetOptions{})
		if err != nil {
			fc.Status.Outgoing.Joined = false
			if slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
				fc.Finalizers = slice.RemoveString(fc.Finalizers, FinalizerString, nil)
			}
			fc, err = r.update(fc)
			if err != nil {
				return nil, err
			}
		}
		return fc, nil
	}

	_, err := foreignDiscoveryClient.Resource("peeringrequests").Get(fc.Status.Outgoing.RemotePeeringRequestName, &metav1.GetOptions{})
	if err != nil {
		fc.Status.Outgoing.Joined = false
		fc.Status.Outgoing.RemotePeeringRequestName = ""
		if slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
			fc.Finalizers = slice.RemoveString(fc.Finalizers, FinalizerString, nil)
		}
		fc, err = r.update(fc)
		if err != nil {
			return nil, err
		}
	}
	return fc, nil
}

// get the external address where the Authentication Service is reachable from the external world.
func (r *ForeignClusterReconciler) getAddress() (string, error) {
	// this address can be overwritten setting this environment variable
	address := r.ConfigProvider.GetConfig().AuthServiceAddress
	if address != "" {
		return address, nil
	}

	// get the authentication service
	svc, err := r.crdClient.Client().CoreV1().Services(r.Namespace).Get(context.TODO(), discovery.AuthServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return "", err
	}
	// if the service is exposed as LoadBalancer
	if svc.Spec.Type == apiv1.ServiceTypeLoadBalancer {
		// get the IP from the LoadBalancer service
		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			// the service has no external IPs
			err := goerrors.New("no valid external IP for LoadBalancer Service")
			klog.Error(err)
			return "", err
		}
		lbIngress := svc.Status.LoadBalancer.Ingress[0]
		// return the external service IP
		if hostname := lbIngress.Hostname; hostname != "" {
			return hostname, nil
		} else if ip := lbIngress.IP; ip != "" {
			return ip, nil
		} else {
			// the service has no external IPs
			err := goerrors.New("no valid external IP for LoadBalancer Service")
			klog.Error(err)
			return "", err
		}
	}

	// only physical nodes
	//
	// we need to get an address from a physical node, if we have established peerings in the past with other clusters,
	// we may have some virtual nodes in our cluster. Since their IPs will not be reachable from other clusters, we cannot use them
	// as address for a local NodePort Service
	labelSelector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      liqoconst.TypeLabel,
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{liqoconst.TypeNode},
			},
		},
	})
	if err != nil {
		klog.Error(err)
		return "", err
	}

	// get the IP from the Nodes, to be used with NodePort services
	nodes, err := r.crdClient.Client().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		klog.Error(err)
		return "", err
	}
	if len(nodes.Items) == 0 {
		// there are no node is the cluster, we cannot get the address on any of them
		err = errors.NewNotFound(schema.GroupResource{
			Group:    apiv1.GroupName,
			Resource: "nodes",
		}, "")
		klog.Error(err)
		return "", err
	}

	node := nodes.Items[0]
	return discoveryPkg.GetAddress(&node)

	// when an error occurs, it means that we was not able to get an address in any of the previous cases:
	// 1. no overwrite variable is set
	// 2. the service is not of type LoadBalancer
	// 3. there are no nodes in the cluster to get the IP for a NodePort service
}

// get the external port where the Authentication Service is reachable from the external world.
func (r *ForeignClusterReconciler) getPort() (string, error) {
	// this port can be overwritten setting this environment variable
	port := r.ConfigProvider.GetConfig().AuthServicePort
	if port != "" {
		return port, nil
	}

	// get the authentication service
	svc, err := r.crdClient.Client().CoreV1().Services(r.Namespace).Get(context.TODO(), discovery.AuthServiceName, metav1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return "", err
	}
	if len(svc.Spec.Ports) == 0 {
		// the service has no available port, we cannot get it
		err = errors.NewNotFound(schema.GroupResource{
			Group:    apiv1.GroupName,
			Resource: string(apiv1.ResourceServices),
		}, discovery.AuthServiceName)
		klog.Error(err)
		return "", err
	}

	if svc.Spec.Type == apiv1.ServiceTypeLoadBalancer {
		// return the LoadBalancer service external port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].Port), nil
	}
	if svc.Spec.Type == apiv1.ServiceTypeNodePort {
		// return the NodePort service port
		return fmt.Sprintf("%v", svc.Spec.Ports[0].NodePort), nil
	}
	// other service types. When we are using an Ingress we should not reach this code, because of the environment variable
	return "",
		fmt.Errorf(
			"you cannot expose the Auth Service with a %v Service. If you are using an Ingress, probably, there are configuration issues",
			svc.Spec.Type)
}

func (r *ForeignClusterReconciler) getHomeAuthURL() (string, error) {
	address, err := r.getAddress()
	if err != nil {
		return "", err
	}

	port, err := r.getPort()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://%s:%v", address, port), nil
}

func (r *ForeignClusterReconciler) createPeeringRequestIfNotExists(remoteClusterID string,
	owner *discoveryv1alpha1.ForeignCluster, foreignClient *crdclient.CRDClient) (*discoveryv1alpha1.PeeringRequest, error) {
	// get config to send to foreign cluster
	fConfig, err := r.getForeignConfig(remoteClusterID, owner)
	if err != nil {
		return nil, err
	}

	localClusterID := r.clusterID.GetClusterID()

	// check if a peering request with our cluster id already exists on remote cluster
	tmp, err := foreignClient.Resource("peeringrequests").Get(localClusterID, &metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) && !utils.IsUnknownAuthority(err) {
		return nil, err
	}
	if utils.IsUnknownAuthority(err) {
		klog.V(4).Info("unknown authority")
		owner.Spec.TrustMode = discoveryPkg.TrustModeUntrusted
		return nil, nil
	}
	pr, ok := tmp.(*discoveryv1alpha1.PeeringRequest)
	inf := errors.IsNotFound(err) || !ok // inf -> IsNotFound
	// if peering request does not exists or its secret was not created for some reason
	if inf || pr.Spec.KubeConfigRef == nil {
		if inf {
			// does not exist -> create new peering request
			authURL, err := r.getHomeAuthURL()
			if err != nil {
				return nil, err
			}
			pr = &discoveryv1alpha1.PeeringRequest{
				TypeMeta: metav1.TypeMeta{
					Kind:       "PeeringRequest",
					APIVersion: "discovery.liqo.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: localClusterID,
				},
				Spec: discoveryv1alpha1.PeeringRequestSpec{
					ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
						ClusterID:   localClusterID,
						ClusterName: r.ConfigProvider.GetConfig().ClusterName,
					},
					Namespace:     r.Namespace,
					KubeConfigRef: nil,
					AuthURL:       authURL,
				},
			}
			tmp, err = foreignClient.Resource("peeringrequests").Create(pr, &metav1.CreateOptions{})
			if err != nil {
				return nil, err
			}
			pr, ok = tmp.(*discoveryv1alpha1.PeeringRequest)
			if !ok {
				return nil, goerrors.New("created object is not a ForeignCluster")
			}
		}
		secret := &apiv1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				// generate name will lead to different names every time, avoiding name collisions
				GenerateName: strings.Join([]string{"pr", r.clusterID.GetClusterID(), ""}, "-"),
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "PeeringRequest",
						Name:       pr.Name,
						UID:        pr.UID,
					},
				},
			},
			StringData: map[string]string{
				"kubeconfig": fConfig,
			},
		}
		secret, err := foreignClient.Client().CoreV1().Secrets(r.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
			// there was an error during secret creation, delete peering request
			err2 := foreignClient.Resource("peeringrequests").Delete(pr.Name, &metav1.DeleteOptions{})
			if err2 != nil {
				klog.Error(err2)
				return nil, err2
			}
			return nil, err
		}
		pr.Spec.KubeConfigRef = &apiv1.ObjectReference{
			Kind:       "Secret",
			Namespace:  secret.Namespace,
			Name:       secret.Name,
			UID:        secret.UID,
			APIVersion: "v1",
		}
		pr.TypeMeta.Kind = "PeeringRequest"
		pr.TypeMeta.APIVersion = "discovery.liqo.io/v1alpha1"
		tmp, err = foreignClient.Resource("peeringrequests").Update(pr.Name, pr, &metav1.UpdateOptions{})
		if err != nil {
			klog.Error(err)
			// delete peering request, also secret will be deleted by garbage collector
			err2 := foreignClient.Resource("peeringrequests").Delete(pr.Name, &metav1.DeleteOptions{})
			if err2 != nil {
				klog.Error(err2)
				return nil, err2
			}
			return nil, err
		}
		pr, ok = tmp.(*discoveryv1alpha1.PeeringRequest)
		if !ok {
			return nil, goerrors.New("created object is not a PeeringRequest")
		}
		return pr, nil
	}
	// already exists
	return pr, nil
}

// this function return a kube-config file to send to foreign cluster and crate everything needed for it.
func (r *ForeignClusterReconciler) getForeignConfig(remoteClusterID string, owner *discoveryv1alpha1.ForeignCluster) (string, error) {
	_, err := r.createClusterRoleIfNotExists(remoteClusterID, owner)
	if err != nil {
		return "", err
	}
	_, err = r.createRoleIfNotExists(remoteClusterID, owner)
	if err != nil {
		return "", err
	}
	sa, err := r.createServiceAccountIfNotExists(remoteClusterID, owner)
	if err != nil {
		return "", err
	}
	_, err = r.createClusterRoleBindingIfNotExists(remoteClusterID, owner)
	if err != nil {
		return "", err
	}
	_, err = r.createRoleBindingIfNotExists(remoteClusterID, owner)
	if err != nil {
		return "", err
	}

	// crdreplicator role binding
	err = r.setDispatcherRole(remoteClusterID, sa, owner)
	if err != nil {
		return "", err
	}

	// check if ServiceAccount already has a secret, wait if not
	if len(sa.Secrets) == 0 {
		wa, err := r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector: "metadata.name=" + remoteClusterID,
		})
		if err != nil {
			return "", err
		}
		timeout := time.NewTimer(500 * time.Millisecond)
		ch := wa.ResultChan()
		defer timeout.Stop()
		defer wa.Stop()
		for iterate := true; iterate; {
			select {
			case s := <-ch:
				_sa := s.Object.(*apiv1.ServiceAccount)
				if _sa.Name == sa.Name && len(_sa.Secrets) > 0 {
					iterate = false
				}
			case <-timeout.C:
				// try to use default config
				if r.ForeignConfig != nil {
					klog.Warning("using default ForeignConfig")
					return r.ForeignConfig.String(), nil
				}
				// ServiceAccount not updated with secrets and no default config
				return "", errors.NewTimeoutError("ServiceAccount's Secret was not created", 0)
			}
		}
	}
	cnf, err := kubeconfig.CreateKubeConfig(r.ConfigProvider, r.crdClient.Client(), remoteClusterID, r.Namespace)
	return cnf, err
}

func (r *ForeignClusterReconciler) createClusterRoleIfNotExists(remoteClusterID string,
	owner *discoveryv1alpha1.ForeignCluster) (*rbacv1.ClusterRole, error) {
	role, err := r.crdClient.Client().RbacV1().ClusterRoles().Get(context.TODO(), remoteClusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		role = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteClusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "create", "update", "delete", "watch"},
					APIGroups: []string{"sharing.liqo.io"},
					Resources: []string{"advertisements", "advertisements/status"},
				},
			},
		}
		return r.crdClient.Client().RbacV1().ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return role, nil
}

func (r *ForeignClusterReconciler) createRoleIfNotExists(
	remoteClusterID string, owner *discoveryv1alpha1.ForeignCluster) (*rbacv1.Role, error) {
	role, err := r.crdClient.Client().RbacV1().Roles(r.Namespace).Get(context.TODO(), remoteClusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteClusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					Verbs:     []string{"get", "list", "create", "update", "delete", "watch"},
					APIGroups: []string{""},
					Resources: []string{"secrets"},
				},
			},
		}
		return r.crdClient.Client().RbacV1().Roles(r.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return role, nil
}

func (r *ForeignClusterReconciler) createServiceAccountIfNotExists(
	remoteClusterID string, owner *discoveryv1alpha1.ForeignCluster) (*apiv1.ServiceAccount, error) {
	sa, err := r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Get(context.TODO(), remoteClusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		sa = &apiv1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteClusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
		}
		return r.crdClient.Client().CoreV1().ServiceAccounts(r.Namespace).Create(context.TODO(), sa, metav1.CreateOptions{})
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return sa, nil
}

func (r *ForeignClusterReconciler) createClusterRoleBindingIfNotExists(
	remoteClusterID string, owner *discoveryv1alpha1.ForeignCluster) (*rbacv1.ClusterRoleBinding, error) {
	rb, err := r.crdClient.Client().RbacV1().ClusterRoleBindings().Get(context.TODO(), remoteClusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		rb = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteClusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      remoteClusterID,
					Namespace: r.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     remoteClusterID,
			},
		}
		return r.crdClient.Client().RbacV1().ClusterRoleBindings().Create(context.TODO(), rb, metav1.CreateOptions{})
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return rb, nil
}

func (r *ForeignClusterReconciler) createRoleBindingIfNotExists(
	remoteClusterID string, owner *discoveryv1alpha1.ForeignCluster) (*rbacv1.RoleBinding, error) {
	rb, err := r.crdClient.Client().RbacV1().RoleBindings(r.Namespace).Get(context.TODO(), remoteClusterID, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		rb = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteClusterID,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      remoteClusterID,
					Namespace: r.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     remoteClusterID,
			},
		}
		return r.crdClient.Client().RbacV1().RoleBindings(r.Namespace).Create(context.TODO(), rb, metav1.CreateOptions{})
	}
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	return rb, nil
}

func (r *ForeignClusterReconciler) setDispatcherRole(remoteClusterID string,
	sa *apiv1.ServiceAccount, owner *discoveryv1alpha1.ForeignCluster) error {
	_, err := r.crdClient.Client().RbacV1().ClusterRoleBindings().Get(
		context.TODO(), remoteClusterID+"-crdreplicator", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// does not exist
		rb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: remoteClusterID + "-crdreplicator",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: fmt.Sprintf("%s/%s", discoveryv1alpha1.GroupVersion.Group, discoveryv1alpha1.GroupVersion.Version),
						Kind:       "ForeignCluster",
						Name:       owner.Name,
						UID:        owner.UID,
					},
				},
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      sa.Name,
					Namespace: sa.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "crdreplicator-role",
			},
		}
		_, err = r.crdClient.Client().RbacV1().ClusterRoleBindings().Create(context.TODO(), rb, metav1.CreateOptions{})
		if err != nil {
			klog.Error(err)
		}
		return err
	}
	if err != nil {
		klog.Error(err)
		return err
	}
	return nil
}

func (r *ForeignClusterReconciler) deleteAdvertisement(fc *discoveryv1alpha1.ForeignCluster) error {
	return fc.DeleteAdvertisement(r.advertisementClient)
}

func (r *ForeignClusterReconciler) deletePeeringRequest(foreignClient *crdclient.CRDClient, fc *discoveryv1alpha1.ForeignCluster) error {
	return foreignClient.Resource("peeringrequests").Delete(fc.Status.Outgoing.RemotePeeringRequestName, &metav1.DeleteOptions{})
}

func (r *ForeignClusterReconciler) getAutoJoin(fc *discoveryv1alpha1.ForeignCluster) bool {
	if r.ConfigProvider == nil || r.ConfigProvider.GetConfig() == nil {
		klog.Warning("Discovery Config is not set, using default value")
		return fc.Spec.Join
	}
	return r.ConfigProvider.GetConfig().AutoJoin
}

func (r *ForeignClusterReconciler) getAutoJoinUntrusted(fc *discoveryv1alpha1.ForeignCluster) bool {
	if r.ConfigProvider == nil || r.ConfigProvider.GetConfig() == nil {
		klog.Warning("Discovery Config is not set, using default value")
		return fc.Spec.Join
	}
	return r.ConfigProvider.GetConfig().AutoJoinUntrusted
}

func (r *ForeignClusterReconciler) checkNetwork(fc *discoveryv1alpha1.ForeignCluster, requireUpdate *bool) error {
	// local NetworkConfig
	labelSelector := strings.Join([]string{crdreplicator.DestinationLabel, fc.Spec.ClusterIdentity.ClusterID}, "=")
	if err := r.updateNetwork(labelSelector, &fc.Status.Network.LocalNetworkConfig, requireUpdate); err != nil {
		klog.Error(err)
		return err
	}

	// remote NetworkConfig
	labelSelector = strings.Join([]string{crdreplicator.RemoteLabelSelector, fc.Spec.ClusterIdentity.ClusterID}, "=")
	return r.updateNetwork(labelSelector, &fc.Status.Network.RemoteNetworkConfig, requireUpdate)
}

func (r *ForeignClusterReconciler) updateNetwork(
	labelSelector string, resourceLink *discoveryv1alpha1.ResourceLink, requireUpdate *bool) error {
	tmp, err := r.networkClient.Resource("networkconfigs").List(&metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	ncfs, ok := tmp.(*netv1alpha1.NetworkConfigList)
	if !ok {
		err = goerrors.New("retrieved object is not a NetworkConfig")
		klog.Error(err)
		return err
	}
	if len(ncfs.Items) == 0 && resourceLink.Available {
		// no NetworkConfigs found
		resourceLink.Available = false
		resourceLink.Reference = nil
		*requireUpdate = true
	} else if len(ncfs.Items) > 0 && !resourceLink.Available {
		// there are NetworkConfigs
		ncf := &ncfs.Items[0]
		resourceLink.Available = true
		resourceLink.Reference = &apiv1.ObjectReference{
			Kind:       "NetworkConfig",
			Name:       ncf.Name,
			UID:        ncf.UID,
			APIVersion: fmt.Sprintf("%s/%s", netv1alpha1.GroupVersion.Group, netv1alpha1.GroupVersion.Version),
		}
		*requireUpdate = true
	}
	return nil
}

func (r *ForeignClusterReconciler) checkTEP(fc *discoveryv1alpha1.ForeignCluster, requireUpdate *bool) error {
	tmp, err := r.networkClient.Resource("tunnelendpoints").List(&metav1.ListOptions{
		LabelSelector: strings.Join([]string{liqoconst.ClusterIDLabelName, fc.Spec.ClusterIdentity.ClusterID}, "="),
	})
	if err != nil {
		klog.Error(err)
		return err
	}
	teps, ok := tmp.(*netv1alpha1.TunnelEndpointList)
	if !ok {
		err = goerrors.New("retrieved object is not a TunnelEndpoint")
		klog.Error(err)
		return err
	}
	if len(teps.Items) == 0 && fc.Status.Network.TunnelEndpoint.Available {
		// no TEP found
		fc.Status.Network.TunnelEndpoint.Available = false
		fc.Status.Network.TunnelEndpoint.Reference = nil
		*requireUpdate = true
	} else if len(teps.Items) > 0 && !fc.Status.Network.TunnelEndpoint.Available {
		// there are TEPs
		tep := &teps.Items[0]
		fc.Status.Network.TunnelEndpoint.Available = true
		fc.Status.Network.TunnelEndpoint.Reference = &apiv1.ObjectReference{
			Kind:       "TunnelEndpoints",
			Name:       tep.Name,
			UID:        tep.UID,
			APIVersion: fmt.Sprintf("%s/%s", netv1alpha1.GroupVersion.Group, netv1alpha1.GroupVersion.Version),
		}
		*requireUpdate = true
	}
	return nil
}
