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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/util/slice"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

// FinalizerString is added as finalizer for peered ForeignClusters.
const FinalizerString = "foreigncluster.discovery.liqo.io/peered"

// ForeignClusterReconciler reconciles a ForeignCluster object.
type ForeignClusterReconciler struct {
	client.Client
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
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=searchdomains,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=peeringrequests,verbs=get;list;watch
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;list;watch;create;update;patch;delete;deletecollection
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
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;create;deletecollection;delete
// role
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=services,verbs=get
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=configmaps,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=serviceaccounts,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=core,namespace="liqo",resources=secrets,verbs=get;list;watch;create;update;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=roles,verbs=get;create;update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,namespace="liqo",resources=rolebindings,verbs=get;create

// Reconcile reconciles ForeignCluster resources.
func (r *ForeignClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	klog.V(4).Infof("Reconciling ForeignCluster %s", req.Name)

	var foreignCluster discoveryv1alpha1.ForeignCluster
	if err := r.Client.Get(ctx, req.NamespacedName, &foreignCluster); err != nil {
		klog.Error(err)
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// ------ (1) validation ------

	// set labels and validate the resource spec
	if cont, res, err := r.validateForeignCluster(ctx, &foreignCluster); !cont {
		return res, err
	}

	if !r.useNewAuth {
		return r.oldReconcile(ctx, &foreignCluster)
	}

	// defer the status update function
	defer func() {
		if newErr := r.Client.Status().Update(ctx, &foreignCluster); newErr != nil {
			klog.Error(newErr)
			err = newErr
		}
	}()

	// ------ (2) ensuring prerequirements ------

	// ensure the existence of the local TenantNamespace
	if err = r.ensureLocalTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ensure the existence of an identity to operate in the remote cluster remote cluster
	if err = r.ensureRemoteIdentity(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// fetch the remote tenant namespace name
	if err = r.fetchRemoteTenantNamespace(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (3) peering/unpeering logic ------

	// read the ForeignCluster status and ensure the peering state
	phase := r.getDesiredOutgoingPeeringState(&foreignCluster)
	switch phase {
	case desiredPeeringPhasePeering:
		if err = r.peerNamespaced(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
	case desiredPeeringPhaseUnpeering:
		if err = r.unpeerNamespaced(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
	default:
		err := fmt.Errorf("unknown phase %v", phase)
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (4) update peering conditions ------

	// check for NetworkConfigs
	if err = r.checkNetwork(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// check for TunnelEndpoints
	if err = r.checkTEP(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// check if peering request really exists on foreign cluster
	if err := r.checkPeeringStatus(ctx, &foreignCluster); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	}

	// ------ (5) garbage collection ------

	// check if this ForeignCluster needs to be deleted. It could happen, for example, if it has been discovered
	// thanks to incoming peeringRequest and it has no active connections
	if foreigncluster.HasToBeRemoved(&foreignCluster) {
		klog.Infof("[%v] Delete ForeignCluster %v with discovery type %v",
			foreignCluster.Spec.ClusterIdentity.ClusterID,
			foreignCluster.Name, foreignCluster.Spec.DiscoveryType)
		if err := r.deleteForeignCluster(ctx, &foreignCluster); err != nil {
			klog.Error(err)
			return ctrl.Result{}, err
		}
		klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
		return ctrl.Result{}, nil
	}

	klog.V(4).Infof("ForeignCluster %s successfully reconciled", foreignCluster.Name)
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

// TODO: delete it
// updateStatus is a utility function that will be deleted after the refactoring completion.
func (r *ForeignClusterReconciler) updateStatus(ctx context.Context, foreignCluster *discoveryv1alpha1.ForeignCluster) (ctrl.Result, error) {
	newStatus := foreignCluster.Status.DeepCopy()
	if result, err := controllerutil.CreateOrPatch(ctx, r.Client, foreignCluster, func() error {
		foreignCluster.Status = *newStatus
		return nil
	}); err != nil {
		klog.Error(err)
		return ctrl.Result{}, err
	} else if result != controllerutil.OperationResultNone {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{Requeue: true, RequeueAfter: r.RequeueAfter}, nil
}

// Peer creates the peering with a remote cluster.
func (r *ForeignClusterReconciler) Peer(
	foreignCluster *discoveryv1alpha1.ForeignCluster,
	foreignDiscoveryClient *crdclient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {

	// create PeeringRequest
	klog.Infof("[%v] Creating PeeringRequest", foreignCluster.Spec.ClusterIdentity.ClusterID)
	pr, err := r.createPeeringRequestIfNotExists(foreignCluster.Name, foreignCluster, foreignDiscoveryClient)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	if pr != nil {
		foreignCluster.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseEstablished
		foreignCluster.Status.Outgoing.RemotePeeringRequestName = pr.Name
		// add finalizer
		controllerutil.AddFinalizer(foreignCluster, FinalizerString)
	}
	return foreignCluster, nil
}

// peerNamespaced enables the peering creating the resources in the correct TenantControlNamespace.
func (r *ForeignClusterReconciler) peerNamespaced(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	// create ResourceRequest
	result, err := r.createResourceRequest(ctx, foreignCluster)
	if err != nil {
		klog.Error(err)
		return err
	}

	// if the resource request has been created
	if result == controllerutil.OperationResultCreated {
		foreignCluster.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhasePending
	}
	return nil
}

// Unpeer removes the peering with a remote cluster.
func (r *ForeignClusterReconciler) Unpeer(fc *discoveryv1alpha1.ForeignCluster,
	foreignDiscoveryClient *crdclient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {

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
	fc.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseNone
	fc.Status.Outgoing.RemotePeeringRequestName = ""
	if slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
		fc.Finalizers = slice.RemoveString(fc.Finalizers, FinalizerString, nil)
	}
	return fc, nil
}

// unpeerNamespaced disables the peering deleting the resources in the correct TenantControlNamespace.
func (r *ForeignClusterReconciler) unpeerNamespaced(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var resourceRequest discoveryv1alpha1.ResourceRequest
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: foreignCluster.Status.TenantControlNamespace.Local,
		Name:      r.clusterID.GetClusterID(),
	}, &resourceRequest)
	if errors.IsNotFound(err) {
		foreignCluster.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseNone
		return nil
	}
	if err != nil {
		klog.Error(err)
		return err
	}

	if resourceRequest.Status.OfferWithdrawalTimestamp.IsZero() {
		if resourceRequest.Spec.WithdrawalTimestamp.IsZero() {
			now := metav1.Now()
			resourceRequest.Spec.WithdrawalTimestamp = &now
		}
		err = r.Client.Update(ctx, &resourceRequest)
		if err != nil && !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}
		foreignCluster.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseDisconnecting
	} else {
		err = r.deleteResourceRequest(ctx, foreignCluster)
		if err != nil && !errors.IsNotFound(err) {
			klog.Error(err)
			return err
		}
		foreignCluster.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseNone
	}
	return nil
}

// SetupWithManager assigns the operator to a manager.
func (r *ForeignClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&discoveryv1alpha1.ForeignCluster{}).
		Owns(&advtypes.Advertisement{}).
		Owns(&discoveryv1alpha1.PeeringRequest{}).
		Owns(&discoveryv1alpha1.ResourceRequest{}).
		Complete(r)
}

func (r *ForeignClusterReconciler) checkPeeringStatus(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var outgoingResourceRequestList discoveryv1alpha1.ResourceRequestList
	remoteClusterID := foreignCluster.Spec.ClusterIdentity.ClusterID
	localNamespace := foreignCluster.Status.TenantControlNamespace.Local
	if err := r.Client.List(ctx, &outgoingResourceRequestList, client.MatchingLabels(resourceRequestLabels(remoteClusterID)),
		client.InNamespace(localNamespace)); err != nil {
		klog.Error(err)
		return err
	}

	var incomingResourceRequestList discoveryv1alpha1.ResourceRequestList
	if err := r.Client.List(ctx, &incomingResourceRequestList, client.HasLabels{
		crdreplicator.ReplicationStatuslabel}, client.MatchingLabels{
		crdreplicator.RemoteLabelSelector: remoteClusterID,
	}); err != nil {
		klog.Error(err)
		return err
	}

	var err error
	foreignCluster.Status.Outgoing.PeeringPhase, err = getPeeringPhase(&outgoingResourceRequestList)
	if err != nil {
		err = fmt.Errorf("[%v] %w in namespace %v", remoteClusterID, err, localNamespace)
		klog.Error(err)
		return err
	}

	foreignCluster.Status.Incoming.PeeringPhase, err = getPeeringPhase(&incomingResourceRequestList)
	if err != nil {
		err = fmt.Errorf("[%v] %w in namespace %v", remoteClusterID, err, localNamespace)
		klog.Error(err)
		return err
	}
	return nil
}

func getPeeringPhase(resourceRequestList *discoveryv1alpha1.ResourceRequestList) (discoveryv1alpha1.PeeringPhaseType, error) {
	switch len(resourceRequestList.Items) {
	case 0:
		return discoveryv1alpha1.PeeringPhaseNone, nil
	case 1:
		resourceRequest := &resourceRequestList.Items[0]
		desiredDelete := !resourceRequest.Spec.WithdrawalTimestamp.IsZero()
		deleted := !resourceRequest.Status.OfferWithdrawalTimestamp.IsZero()
		if desiredDelete && !deleted {
			return discoveryv1alpha1.PeeringPhaseDisconnecting, nil
		}
		return discoveryv1alpha1.PeeringPhaseEstablished, nil
	default:
		err := fmt.Errorf("more than one resource request found")
		return discoveryv1alpha1.PeeringPhaseNone, err
	}
}

// checkJoined checks the outgoing join status of a cluster.
func (r *ForeignClusterReconciler) checkJoined(fc *discoveryv1alpha1.ForeignCluster,
	foreignDiscoveryClient *crdclient.CRDClient) (*discoveryv1alpha1.ForeignCluster, error) {

	_, err := foreignDiscoveryClient.Resource("peeringrequests").Get(fc.Status.Outgoing.RemotePeeringRequestName, &metav1.GetOptions{})
	if err != nil {
		fc.Status.Outgoing.PeeringPhase = discoveryv1alpha1.PeeringPhaseNone
		fc.Status.Outgoing.RemotePeeringRequestName = ""
		if slice.ContainsString(fc.Finalizers, FinalizerString, nil) {
			fc.Finalizers = slice.RemoveString(fc.Finalizers, FinalizerString, nil)
		}
		status := fc.Status.DeepCopy()
		fc, err = r.update(fc)
		if err != nil {
			return nil, err
		}
		fc.Status = *status
		_, err = r.updateStatus(context.TODO(), fc)
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
	var adv advtypes.Advertisement
	if err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name: fmt.Sprintf("advertisement-%s", fc.Spec.ClusterIdentity.ClusterID)}, &adv); errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		klog.Error(err)
		return err
	}
	return r.Client.Delete(context.TODO(), &adv)
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

func (r *ForeignClusterReconciler) checkNetwork(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	// local NetworkConfig
	labelSelector := map[string]string{crdreplicator.DestinationLabel: foreignCluster.Spec.ClusterIdentity.ClusterID}
	if err := r.updateNetwork(ctx, labelSelector, &foreignCluster.Status.Network.LocalNetworkConfig); err != nil {
		klog.Error(err)
		return err
	}

	// remote NetworkConfig
	labelSelector = map[string]string{crdreplicator.RemoteLabelSelector: foreignCluster.Spec.ClusterIdentity.ClusterID}
	if err := r.updateNetwork(ctx, labelSelector, &foreignCluster.Status.Network.RemoteNetworkConfig); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (r *ForeignClusterReconciler) updateNetwork(ctx context.Context,
	labelSelector map[string]string, resourceLink *discoveryv1alpha1.ResourceLink) error {
	var netList netv1alpha1.NetworkConfigList
	if err := r.Client.List(ctx, &netList, client.MatchingLabels(labelSelector)); err != nil {
		klog.Error(err)
		return err
	}
	if len(netList.Items) == 0 {
		// no NetworkConfigs found
		resourceLink.Available = false
		resourceLink.Reference = nil
	} else if len(netList.Items) > 0 {
		// there are NetworkConfigs
		ncf := &netList.Items[0]
		resourceLink.Available = true
		resourceLink.Reference = &apiv1.ObjectReference{
			Kind:       "NetworkConfig",
			Name:       ncf.Name,
			UID:        ncf.UID,
			APIVersion: fmt.Sprintf("%s/%s", netv1alpha1.GroupVersion.Group, netv1alpha1.GroupVersion.Version),
		}
	}
	return nil
}

func (r *ForeignClusterReconciler) checkTEP(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	var tepList netv1alpha1.TunnelEndpointList
	if err := r.Client.List(ctx, &tepList, client.MatchingLabels{
		liqoconst.ClusterIDLabelName: foreignCluster.Spec.ClusterIdentity.ClusterID,
	}); err != nil {
		klog.Error(err)
		return err
	}

	if len(tepList.Items) == 0 {
		foreignCluster.Status.Network.TunnelEndpoint.Available = false
		foreignCluster.Status.Network.TunnelEndpoint.Reference = nil
	} else if len(tepList.Items) > 0 {
		tep := &tepList.Items[0]
		foreignCluster.Status.Network.TunnelEndpoint.Available = true
		foreignCluster.Status.Network.TunnelEndpoint.Reference = &apiv1.ObjectReference{
			Kind:       "TunnelEndpoints",
			Name:       tep.Name,
			UID:        tep.UID,
			APIVersion: fmt.Sprintf("%s/%s", netv1alpha1.GroupVersion.Group, netv1alpha1.GroupVersion.Version),
		}
	}
	return nil
}

func (r *ForeignClusterReconciler) deleteForeignCluster(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) error {
	return r.Client.Delete(ctx, foreignCluster)
}

func (r *ForeignClusterReconciler) removeFinalizer(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster) (controllerutil.OperationResult, error) {
	return controllerutil.CreateOrUpdate(ctx, r.Client, foreignCluster, func() error {
		controllerutil.RemoveFinalizer(foreignCluster, FinalizerString)
		return nil
	})
}
