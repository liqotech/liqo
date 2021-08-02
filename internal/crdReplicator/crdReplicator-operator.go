package crdreplicator

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

var (
	ResyncPeriod = 30 * time.Second
	result       = ctrl.Result{
		RequeueAfter: 30 * time.Second,
	}
)

// ReplicatedResourcesLabelSelector is an helper label selector to list all the replicated resources.
var ReplicatedResourcesLabelSelector = metav1.LabelSelector{
	MatchExpressions: []metav1.LabelSelectorRequirement{
		{
			Key:      RemoteLabelSelector,
			Operator: metav1.LabelSelectorOpExists,
		},
		{
			Key:      ReplicationStatuslabel,
			Operator: metav1.LabelSelectorOpExists,
		},
	},
}

const (
	operatorName           = "crdReplicator-operator"
	finalizer              = "crdReplicator.liqo.io"
	LocalLabelSelector     = "liqo.io/replication"
	RemoteLabelSelector    = "liqo.io/originID"
	DestinationLabel       = "liqo.io/remoteID"
	ReplicationStatuslabel = "liqo.io/replicated"
)

type Controller struct {
	Scheme *runtime.Scheme
	client.Client
	ClientSet                      *kubernetes.Clientset
	ClusterID                      string
	RemoteDynClients               map[string]dynamic.Interface                            // for each remote cluster we save dynamic client connected to its API server
	RemoteDynSharedInformerFactory map[string]dynamicinformer.DynamicSharedInformerFactory // for each remote cluster we save the dynamic shared informer factory
	LocalDynClient                 dynamic.Interface                                       // dynamic client pointing to the local API server
	LocalDynSharedInformerFactory  dynamicinformer.DynamicSharedInformerFactory            // local dynamic shared informer factory
	// RegisteredResources is a list of GVRs of resources to be replicated, with the associated peering phase when the replication has to occur.
	RegisteredResources []configv1alpha1.Resource
	// UnregisteredResources, each time a resource is removed from the configuration it is saved in this list,
	// it stays here until the associated watcher, if running, is stopped.
	UnregisteredResources []metav1.GroupVersionResource
	// LocalWatchers, we save all the running watchers monitoring the local resources:(registeredResource, chan)).
	LocalWatchers map[string]chan struct{}
	// RemoteWatchers, for each peering cluster we save all the running watchers monitoring the replicated resources:
	// (clusterID, (registeredResource, chan)).
	RemoteWatchers map[string]map[string]chan struct{}

	// NamespaceManager is an interface to manage the tenant namespaces.
	NamespaceManager tenantnamespace.Manager
	// IdentityManager is an interface to manage remote identities, and to get the rest config.
	IdentityManager identitymanager.IdentityManager
	// LocalToRemoteNamespaceMapper maps local namespaces to remote ones.
	LocalToRemoteNamespaceMapper map[string]string
	// RemoteToLocalNamespaceMapper maps remote namespaces to local ones.
	RemoteToLocalNamespaceMapper map[string]string
	// ClusterIDToLocalNamespaceMapper maps clusterIDs to local namespaces.
	ClusterIDToLocalNamespaceMapper map[string]string
	// ClusterIDToRemoteNamespaceMapper maps clusterIDs to remote namespaces.
	ClusterIDToRemoteNamespaceMapper map[string]string
	peeringPhases                    map[string]consts.PeeringPhase
	peeringPhasesMutex               sync.RWMutex
}

// cluster-role
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=config.liqo.io,resources=clusterconfigs,verbs=get;list;watch
// role
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=secrets,verbs=get;list
// +kubebuilder:rbac:groups=core,namespace="do-not-care",resources=configmaps,verbs=get;list

// identity management
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list

// Reconcile handles requests for subscribed types of object.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var fc discoveryv1alpha1.ForeignCluster
	c.StartWatchers()
	defer c.StopWatchers()
	err := c.Get(ctx, req.NamespacedName, &fc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("%s -> unable to retrieve resource %s: %s", c.ClusterID, req.NamespacedName, err)
		return result, err
	}
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	}
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	// examine DeletionTimestamp to determine if object is under deletion
	if fc.ObjectMeta.DeletionTimestamp.IsZero() {
		// the finalizer is added only if a join is active with the remote cluster
		if !foreigncluster.IsIncomingEnabled(&fc) && !foreigncluster.IsOutgoingEnabled(&fc) {
			if err := c.updateForeignCluster(ctx, &fc, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", fc.Name, err)
				return result, err
			}
		} else if err := c.updateForeignCluster(ctx, &fc, controllerutil.AddFinalizer); err != nil {
			klog.Errorf("%s -> unable to update resource %s: %s", c.ClusterID, fc.Name, err)
			return result, err
		}
	} else {
		// the object is being deleted
		if controllerutil.ContainsFinalizer(&fc, finalizer) {
			// close remote watcher for remote cluster
			_, ok := c.RemoteWatchers[remoteClusterID]
			if ok {
				for i := range c.RegisteredResources {
					klog.Infof("%s -> closing remote watcher for resource %s",
						remoteClusterID, c.RegisteredResources[i].GroupVersionResource.String())
					if err = c.cleanupRemoteWatcher(remoteClusterID, c.RegisteredResources[i].GroupVersionResource); err != nil {
						klog.Error(err)
						continue
					}
				}
				delete(c.RemoteWatchers, remoteClusterID)
			}

			// delete dynamic client for remote cluster
			delete(c.RemoteDynClients, remoteClusterID)
			// delete informer for remote cluster
			delete(c.RemoteDynSharedInformerFactory, remoteClusterID)
			// remove the finalizer from the list and update it.
			if err := c.updateForeignCluster(ctx, &fc, controllerutil.RemoveFinalizer); err != nil {
				klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", fc.Name, err)
				return result, err
			}
			return result, nil
		}
	}

	currentPhase := foreigncluster.GetPeeringPhase(&fc)
	if oldPhase := c.getPeeringPhase(remoteClusterID); oldPhase != currentPhase {
		c.setPeeringPhase(remoteClusterID, currentPhase)
		defer c.checkResourcesOnPeeringPhaseChange(ctx, remoteClusterID, currentPhase, oldPhase)
	}

	// check if the client already exists
	// check if the dynamic dynamic client and informer factory exists
	_, dynClientOk := c.RemoteDynClients[remoteClusterID]
	_, dynFacOk := c.RemoteDynSharedInformerFactory[remoteClusterID]
	if dynClientOk && dynFacOk {
		return result, nil
	}

	if fc.Status.TenantNamespace.Local == "" || fc.Status.TenantNamespace.Remote == "" {
		klog.V(4).Infof("%s -> tenantNamespace is not set in resource %s for remote peering cluster %s",
			c.ClusterID, req.NamespacedName, remoteClusterID)
		return result, nil
	}
	config, err := c.IdentityManager.GetConfig(remoteClusterID, fc.Status.TenantNamespace.Local)
	if err != nil {
		klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s",
			c.ClusterID, req.NamespacedName, remoteClusterID, err)
		return result, nil
	}

	c.setUpConnectionToPeeringCluster(config, &fc)
	return result, nil
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).Named(operatorName).WithEventFilter(resourceToBeProccesedPredicate).
		For(&discoveryv1alpha1.ForeignCluster{}).
		Complete(c)
}

// updateForeignCluster updates the ForeignCluster applying the provided function. It is supposed to add/remove finalizers.
// It applies a longer exponential backoff on conflicts to avoid infinite (or long) conflict chain with the main
// ForeignCluster operator.
func (c *Controller) updateForeignCluster(ctx context.Context,
	foreignCluster *discoveryv1alpha1.ForeignCluster, f func(client.Object, string)) error {
	backoff := retry.DefaultBackoff
	backoff.Duration = 100 * time.Millisecond
	return retry.RetryOnConflict(backoff, func() error {
		if err := c.Client.Get(ctx, types.NamespacedName{Name: foreignCluster.Name}, foreignCluster); err != nil {
			klog.Error(err)
			return err
		}
		f(foreignCluster, finalizer)
		return c.Client.Update(ctx, foreignCluster)
	})
}

func (c *Controller) setUpTranslations(fc *discoveryv1alpha1.ForeignCluster) {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	localNamespace := fc.Status.TenantNamespace.Local
	remoteNamespace := fc.Status.TenantNamespace.Remote
	keyedRemoteNamespace := remoteNamespaceKeyer(remoteClusterID, remoteNamespace)
	klog.V(3).Infof("%s -> setting up mapping local namespace %s to %s",
		remoteClusterID, localNamespace, remoteNamespace)
	c.LocalToRemoteNamespaceMapper[localNamespace] = remoteNamespace

	klog.V(3).Infof("%s -> setting up mapping remote namespace %s to %s",
		remoteClusterID, remoteNamespace, localNamespace)
	c.RemoteToLocalNamespaceMapper[keyedRemoteNamespace] = localNamespace

	c.ClusterIDToLocalNamespaceMapper[remoteClusterID] = localNamespace
	c.ClusterIDToRemoteNamespaceMapper[remoteClusterID] = remoteNamespace
}

func (c *Controller) setUpConnectionToPeeringCluster(config *rest.Config, fc *discoveryv1alpha1.ForeignCluster) {
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	remoteNamespace := fc.Status.TenantNamespace.Remote

	c.setUpTranslations(fc)

	// check if the dynamic dynamic client exists
	if _, ok := c.RemoteDynClients[remoteClusterID]; !ok {
		dynClient, err := dynamic.NewForConfig(config)
		if err != nil {
			klog.Errorf("%s -> unable to create dynamic client in order to create the dynamic shared informer factory: %s", remoteClusterID, err)
			// we don't need to immediately requeue the foreign cluster but wait for the next re-sync
			return
		} else {
			klog.Infof("%s -> dynamic client created", remoteClusterID)
		}
		c.RemoteDynClients[remoteClusterID] = dynClient
	}
	// check if the dynamic shared informer factory exists
	if _, ok := c.RemoteDynSharedInformerFactory[remoteClusterID]; !ok {
		f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
			c.RemoteDynClients[remoteClusterID], ResyncPeriod, remoteNamespace, c.SetLabelsForRemoteResources)
		c.RemoteDynSharedInformerFactory[remoteClusterID] = f
		klog.Infof("%s -> dynamic shared informer factory created", remoteClusterID)
	}
}

func (c *Controller) SetLabelsForRemoteResources(options *metav1.ListOptions) {
	// we want to watch only the resources that have been created by us on the remote cluster
	if options.LabelSelector == "" {
		newLabelSelector := []string{RemoteLabelSelector, "=", c.ClusterID}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	} else {
		newLabelSelector := []string{options.LabelSelector, RemoteLabelSelector, "=", c.ClusterID}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}

func SetLabelsForLocalResources(options *metav1.ListOptions) {
	// we want to watch only the resources that should be replicated on a remote cluster
	if options.LabelSelector == "" {
		newLabelSelector := []string{LocalLabelSelector, "=", "true"}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	} else {
		newLabelSelector := []string{options.LabelSelector, LocalLabelSelector, "=", "true"}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}

func (c *Controller) Watcher(dynFac dynamicinformer.DynamicSharedInformerFactory, gvr schema.GroupVersionResource, handlerFuncs cache.ResourceEventHandlerFuncs, stopCh chan struct{}) {
	// get informer for resource
	inf := dynFac.ForResource(gvr)
	inf.Informer().AddEventHandler(handlerFuncs)
	inf.Informer().Run(stopCh)
}

func (c *Controller) getGVR(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	resource := strings.ToLower(gvk.Kind)
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resource + "s",
	}
	return gvr
}

func (c *Controller) remoteAddWrapper(obj interface{}) {
	c.remoteModifiedWrapper(nil, obj)
}

func (c *Controller) remoteModifiedWrapper(oldObj, newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := c.getGVR(objUnstruct)
	remoteClusterID := objUnstruct.GetLabels()[DestinationLabel]
	resource := c.getResource(&gvr)
	if resource == nil {
		return
	}

	c.RemoteResourceModifiedHandler(objUnstruct.DeepCopy(), gvr, remoteClusterID, resource.Ownership)
}

// RemoteResourceModifiedHandler handles updates on a remote resource, updating the local status if it is
// in a shared ownership or forcing the remote status if the resource is only owned by the local cluster.
func (c *Controller) RemoteResourceModifiedHandler(obj *unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	remoteClusterID string,
	ownership consts.OwnershipType) {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	localNamespace := c.remoteToLocalNamespace(remoteClusterID, namespace)

	localDynClient := c.LocalDynClient
	clusterID := c.ClusterID
	// we check if the resource exists in the local cluster
	localObj, found, err := c.GetResource(localDynClient, gvr, name, localNamespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return
	}
	// TODO if the resource does not exist what do we do?
	// do nothing? remove the remote replication?
	// current choice -> remove the remote one as well
	if !found {
		klog.Infof("%s -> resource %s in namespace %s of type %s not found", clusterID, name, localNamespace, gvr.String())
		klog.Infof("%s -> removing resource %s in namespace %s of type %s", remoteClusterID, name, localNamespace, gvr.String())
		err := c.DeleteResource(c.RemoteDynClients[remoteClusterID], gvr, obj, remoteClusterID)
		if err != nil {
			return
		}
		return
	}
	// if the resource exists on the local cluster then we update the status

	// we reflect on the local resource only the changes of the status of the remote one
	// we do not reflect the changes to labels, annotations or spec
	// TODO:support labels and annotations

	// get status field for both resources
	remoteStatus, err := getStatus(obj, clusterID)
	if err != nil {
		return
	}
	if remoteStatus == nil {
		return
	}
	localStatus, err := getStatus(localObj, remoteClusterID)
	if err != nil {
		return
	}
	// check if are the same and if not update the local one
	if !reflect.DeepEqual(localStatus, remoteStatus) {
		klog.Infof("%s -> updating status field of resource %s of type %s", clusterID, name, gvr.String())
		switch ownership {
		case consts.OwnershipShared:
			// copy the remote status to the local object
			if err = c.UpdateStatus(localDynClient, gvr, localObj, clusterID, remoteStatus); err != nil {
				klog.Error(err)
			}
			return
		case consts.OwnershipLocal:
			// copy the local status to the remote object
			if err = c.UpdateStatus(c.RemoteDynClients[remoteClusterID], gvr, obj, remoteClusterID, localStatus); err != nil {
				klog.Error(err)
			}
			return
		default:
			err := fmt.Errorf("unknown OwnershipType %v", ownership)
			klog.Error(err)
			return
		}
	}
}

func (c *Controller) StartWatchers() {
	// for each remote cluster check if the remote watchers are running for each registered resource
	for remCluster, remDynFac := range c.RemoteDynSharedInformerFactory {
		watchers := c.RemoteWatchers[remCluster]
		if watchers == nil {
			watchers = make(map[string]chan struct{})
		}
		for i := range c.RegisteredResources {
			res := &c.RegisteredResources[i]
			if !foreigncluster.IsReplicationEnabled(c.getPeeringPhase(remCluster), res) {
				continue
			}

			gvr := convertGVR(res.GroupVersionResource)
			// if there is not then start one
			if _, ok := watchers[gvr.String()]; !ok {
				stopCh := make(chan struct{})
				watchers[gvr.String()] = stopCh
				go c.Watcher(remDynFac, gvr, cache.ResourceEventHandlerFuncs{
					AddFunc:    c.remoteAddWrapper,
					UpdateFunc: c.remoteModifiedWrapper,
				}, stopCh)
				klog.Infof("%s -> starting remote watcher for resource: %s", remCluster, gvr.String())
			}
		}
		c.RemoteWatchers[remCluster] = watchers
	}
	// check if the local watchers are running for each registered resource
	for _, res := range c.RegisteredResources {
		gvr := convertGVR(res.GroupVersionResource)
		// if there is not a running local watcher then start one
		if _, ok := c.LocalWatchers[gvr.String()]; !ok {
			stopCh := make(chan struct{})
			c.LocalWatchers[gvr.String()] = stopCh
			go c.Watcher(c.LocalDynSharedInformerFactory, gvr, cache.ResourceEventHandlerFuncs{
				AddFunc:    c.AddFunc,
				UpdateFunc: c.UpdateFunc,
				DeleteFunc: c.DeleteFunc,
			}, stopCh)
			klog.Infof("%s -> starting local watcher for resource: %s", c.ClusterID, gvr.String())
		}
	}
}

// Stops all the watchers for the resources that have been unregistered.
func (c *Controller) StopWatchers() {
	// stop all remote watchers for unregistered resources
	for remCluster, watchers := range c.RemoteWatchers {
		for _, res := range c.UnregisteredResources {
			if _, ok := watchers[res.String()]; ok {
				if err := c.cleanupRemoteWatcher(remCluster, res); err != nil {
					klog.Error(err)
					continue
				}
				klog.Infof("%s -> stopping remote watcher for resource: %s", remCluster, res)
			}
		}

		// stop watchers for those resources no more needed
		for i := range c.RegisteredResources {
			res := &c.RegisteredResources[i]
			if foreigncluster.IsReplicationEnabled(c.getPeeringPhase(remCluster), res) {
				continue
			}

			if _, ok := watchers[res.GroupVersionResource.String()]; ok {
				if err := c.cleanupRemoteWatcher(remCluster, res.GroupVersionResource); err != nil {
					klog.Error(err)
					continue
				}
				klog.Infof("%s -> stopping remote watcher for resource: %s", remCluster, res)
			}
		}

		c.RemoteWatchers[remCluster] = watchers
	}
	// stop all local watchers
	for i := range c.UnregisteredResources {
		res := &c.UnregisteredResources[i]
		if ch, ok := c.LocalWatchers[res.String()]; ok {
			close(ch)
			delete(c.LocalWatchers, res.String())
			klog.Infof("%s -> stopping local watcher for resource: %s", c.ClusterID, res)
		}
	}
}

func (c *Controller) cleanupRemoteWatcher(remoteClusterID string, groupVersionResource metav1.GroupVersionResource) error {
	if ch, ok := c.RemoteWatchers[remoteClusterID][groupVersionResource.String()]; !ok || !isChanOpen(ch) {
		return nil
	}
	close(c.RemoteWatchers[remoteClusterID][groupVersionResource.String()])
	delete(c.RemoteWatchers[remoteClusterID], groupVersionResource.String())

	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			LocalLabelSelector:     "false",
			ReplicationStatuslabel: "true",
			RemoteLabelSelector:    c.ClusterID,
		},
	}

	namespace, err := c.clusterIDToRemoteNamespace(remoteClusterID)
	if err != nil {
		klog.Error(err)
		return err
	}

	resourceClient := c.RemoteDynClients[remoteClusterID].Resource(convertGVR(groupVersionResource))
	if err := resourceClient.Namespace(namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(selector.MatchLabels).String(),
	}); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

// CreateResource creates the object with the provided dynamicClient.
func (c *Controller) CreateResource(dynClient dynamic.Interface,
	gvr schema.GroupVersionResource,
	obj *unstructured.Unstructured,
	clusterID string,
	ownership consts.OwnershipType) error {
	// check if the resource exists
	name := obj.GetName()
	namespace := obj.GetNamespace()

	klog.Infof("%s -> creating resource %s of type %s", clusterID, name, gvr.String())
	r, found, err := c.GetResource(dynClient, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return err
	}
	if found {
		// the resource already exists check if the resources are the same
		var compare func(*unstructured.Unstructured, *unstructured.Unstructured) bool
		switch ownership {
		case consts.OwnershipLocal:
			compare = areEqual
		case consts.OwnershipShared:
			compare = areSpecEqual
		default:
			err := fmt.Errorf("unknown OwnershipType %v", ownership)
			klog.Error(err)
			return err
		}
		if compare(obj, r) {
			klog.Infof("%s -> resource %s of type %s already exists", clusterID, obj.GetName(), gvr.String())
			return nil
		} else {
			// if not equal delete the remote one
			err := dynClient.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("%s -> an error occurred while deleting resource %s: %s", clusterID, name, err)
				return err
			}
		}
	}
	// if we come here it means that we have to create the resource on the remote cluster
	spec, b, err := unstructured.NestedMap(obj.Object, "spec")
	if !b || err != nil {
		klog.Errorf("%s -> an error occurred while processing the 'spec' of the resource %s: %s ", c.ClusterID, name, err)
		return err
	}
	remRes := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": obj.GetAPIVersion(),
			"kind":       obj.GetKind(),
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    c.UpdateLabels(obj.GetLabels()),
			},
			"spec": spec,
		},
	}
	// create the resource on the remote cluster
	_, err = dynClient.Resource(gvr).Namespace(namespace).Create(context.TODO(), remRes, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating the resource %s %s of type %s: %s", clusterID, name, namespace, gvr.String(), err)
		return err
	}
	return err
}

func (c *Controller) UpdateLabels(labels map[string]string) map[string]string {
	// we don't check if the map is nil, because it has to be initialized because we use the label to filter the resources
	// which needs to be replicated
	// setting the replication label to false
	labels[LocalLabelSelector] = "false"
	// setting replication status to true
	labels[ReplicationStatuslabel] = "true"
	// setting originID i.e clusterID of home cluster
	labels[RemoteLabelSelector] = c.ClusterID
	return labels
}

// checks if the spec of two resources are the same.
func areSpecEqual(local, remote *unstructured.Unstructured) bool {
	localSpec, b, err := unstructured.NestedMap(local.Object, "spec")
	if !b || err != nil {
		return false
	}
	remoteSpec, b, err := unstructured.NestedMap(remote.Object, "spec")
	if !b || err != nil {
		return false
	}
	return reflect.DeepEqual(localSpec, remoteSpec)
}

// checks if the spec and status of two resources are the same.
func areEqual(local, remote *unstructured.Unstructured) bool {
	if !areSpecEqual(local, remote) {
		return false
	}

	localStatus, b1, err := unstructured.NestedMap(local.Object, "status")
	if err != nil {
		return false
	}
	remoteStatus, b2, err := unstructured.NestedMap(remote.Object, "status")
	if err != nil {
		return false
	}
	if b1 != b2 {
		return false
	}

	return reflect.DeepEqual(localStatus, remoteStatus)
}

func getSpec(obj *unstructured.Unstructured, clusterID string) (map[string]interface{}, error) {
	spec, b, err := unstructured.NestedMap(obj.Object, "spec")
	if !b {
		return nil, nil
	}
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting the spec field from resource %s %s of type %s", clusterID, obj.GetName(), obj.GetKind(), err)
		return nil, err
	}
	return spec, nil
}

func getStatus(obj *unstructured.Unstructured, clusterID string) (map[string]interface{}, error) {
	status, b, err := unstructured.NestedMap(obj.Object, "status")
	if !b {
		return nil, nil
	}
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting the status field from resource %s %s  of type %s: %s", clusterID, obj.GetName(), obj.GetNamespace(), obj.GetKind(), err)
		return nil, err
	}
	return status, nil
}

func (c *Controller) GetResource(client dynamic.Interface, gvr schema.GroupVersionResource, name, namespace, clusterID string) (*unstructured.Unstructured, bool, error) {
	r, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return r, true, nil
	} else if apierrors.IsNotFound(err) {
		return nil, false, nil
	} else {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return nil, false, err
	}
}

func (c *Controller) AddFunc(newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := c.getGVR(objUnstruct)
	c.AddedHandler(objUnstruct.DeepCopy(), gvr)
}

func (c *Controller) AddedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	// check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", c.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}

	resource := c.getResource(&gvr)
	if resource == nil || !foreigncluster.IsReplicationEnabled(c.getPeeringPhase(remoteClusterID), resource) {
		return
	}

	remoteNamespace := c.localToRemoteNamespace(obj.GetNamespace())
	obj.SetNamespace(remoteNamespace)

	if dynClient, ok := c.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", c.ClusterID, remoteClusterID)
		return
	} else {
		err := c.CreateResource(dynClient, gvr, obj, remoteClusterID, resource.Ownership)
		if err != nil {
			klog.Error(err)
		}
	}
}

func (c *Controller) UpdateFunc(oldObj, newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	klog.V(4).Infof("triggered on update %v/%v", objUnstruct.GetNamespace(), objUnstruct.GetName())
	gvr := c.getGVR(objUnstruct)
	c.ModifiedHandler(objUnstruct.DeepCopy(), gvr)
}

func (c *Controller) ModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	// check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", c.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}

	resource := c.getResource(&gvr)
	if resource == nil || !foreigncluster.IsReplicationEnabled(c.getPeeringPhase(remoteClusterID), resource) {
		return
	}

	if dynClient, ok := c.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", c.ClusterID, remoteClusterID)
		return
	} else {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		clusterID := remoteClusterID

		namespace = c.localToRemoteNamespace(namespace)
		obj.SetNamespace(namespace)

		// we check if the resource exists in the remote cluster
		_, found, err := c.GetResource(dynClient, gvr, name, namespace, clusterID)
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
		// if the resource does not exist then we create it
		if !found {
			err := c.CreateResource(dynClient, gvr, obj, clusterID, resource.Ownership)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				klog.Error(err)
				return
			}
		}
		// if the resource exists or we just created it then we update the fields
		// we do this considering that the resource existed, even if we just created it
		if err = c.UpdateResource(dynClient, gvr, obj, clusterID, resource.Ownership); err != nil {
			klog.Errorf("%s -> an error occurred while updating resource %s/%s of type %s: %s", clusterID, namespace, name, gvr.String(), err)
			return
		}
	}
}

func (c *Controller) DeleteFunc(newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := c.getGVR(objUnstruct)
	c.DeletedHandler(objUnstruct.DeepCopy(), gvr)
}

func (c *Controller) DeletedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	// check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", c.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}

	if dynClient, ok := c.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", c.ClusterID, remoteClusterID)
		return
	} else {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		dynClient := dynClient
		clusterID := remoteClusterID

		namespace = c.localToRemoteNamespace(namespace)
		obj.SetNamespace(namespace)

		// we check if the resource exists in the remote cluster
		_, found, err := c.GetResource(dynClient, gvr, name, namespace, clusterID)
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
		// if the resource exists on the remote cluster then we delete it
		if found {
			err := c.DeleteResource(dynClient, gvr, obj, clusterID)
			if err != nil {
				klog.Error(err)
			}
		}
	}
}

// UpdateResource updates the object with the provided client.
// If the ownership is shared the status is ignored, if the owner is the local
// cluster is the owner, the local status is forced on the remote resource.
func (c *Controller) UpdateResource(dynClient dynamic.Interface,
	gvr schema.GroupVersionResource,
	obj *unstructured.Unstructured,
	clusterID string,
	ownership consts.OwnershipType) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	// Retrieve the latest version of resource before attempting update
	r, found, err := c.GetResource(dynClient, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return err
	}
	// this one should never happen, if it does then someone deleted the resource on the other cluster
	if !found {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return fmt.Errorf("something strange happened, check if the resource %s of type %s on cluster %s exists on the remote cluster", name, gvr.String(), clusterID)
	}
	// get spec fields for both resources
	localSpec, err := getSpec(obj, c.ClusterID)
	if err != nil {
		return err
	}
	remoteSpec, err := getSpec(r, clusterID)
	if err != nil {
		return err
	}
	// check if are the same and if not update the remote one
	if !reflect.DeepEqual(localSpec, remoteSpec) {
		klog.Infof("%s -> updating spec field of resource %s of type %s", clusterID, name, gvr.String())
		err := c.UpdateSpec(dynClient, gvr, r, clusterID, localSpec)
		if err != nil {
			return err
		}
	}

	switch ownership {
	case consts.OwnershipShared:
		return nil
	case consts.OwnershipLocal:
		// get status field for both resources
		localStatus, err := getStatus(obj, c.ClusterID)
		if err != nil {
			return err
		}
		if localStatus == nil {
			return nil
		}
		remoteStatus, err := getStatus(r, clusterID)
		if err != nil {
			return err
		}
		// check if are the same and if not update the remote one
		if !reflect.DeepEqual(localStatus, remoteStatus) {
			klog.Infof("%s -> updating status field resource %s of type %s", clusterID, name, gvr.String())
			err := c.UpdateStatus(dynClient, gvr, r, clusterID, localStatus)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		err := fmt.Errorf("unknown OwnershipType %v", ownership)
		klog.Error(err)
		return err
	}
}

func (c *Controller) DeleteResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	klog.Infof("%s -> deleting resource %s of type %s", clusterID, obj.GetName(), gvr.String())
	err := client.Resource(gvr).Namespace(obj.GetNamespace()).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("%s -> an error occurred while deleting resource %s of type %s: %s", clusterID, obj.GetName(), gvr.String(), err)
		return err
	}
	return nil
}

// updates the spec field of a resource.
func (c *Controller) UpdateSpec(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, spec map[string]interface{}) error {
	res := &unstructured.Unstructured{}
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// get the latest version of the resource before attempting to update it
		res, err := client.Resource(gvr).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting the latest version of resource %s %s of kind %s before attempting to update its spec field: %s", clusterID, obj.GetName(), obj.GetNamespace(), gvr.String(), err)
			return err
		}
		// setting the new values of spec fields
		err = unstructured.SetNestedMap(res.Object, spec, "spec")
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting the new spec fields of resource %s %s of kind %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), err)
			return err
		}
		// update the remote resource
		_, err = client.Resource(gvr).Namespace(obj.GetNamespace()).Update(context.TODO(), res, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		return nil
	})
	if retryError != nil {
		klog.Errorf("%s -> an error occurred while updating spec field of resource %s %s of type %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), retryError)
		return retryError
	}
	return nil
}

// updates the status field of a resource.
func (c *Controller) UpdateStatus(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, status map[string]interface{}) error {
	res := &unstructured.Unstructured{}
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// get the latest version of the resource before attempting to update it
		res, err := client.Resource(gvr).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting the latest version of resource %s %s of kind %s before attempting to update its status field: %s", clusterID, obj.GetName(), obj.GetNamespace(), gvr.String(), err)
			return err
		}
		// setting the new values of status fields
		err = unstructured.SetNestedMap(res.Object, status, "status")
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting the new status fields of resource %s %s of kind %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), err)
			return err
		}
		// update the remote resource
		_, err = client.Resource(gvr).Namespace(obj.GetNamespace()).UpdateStatus(context.TODO(), res, metav1.UpdateOptions{})
		return err
	})
	if retryError != nil {
		klog.Errorf("%s -> an error occurred while updating status field of resource %s %s of type %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), retryError)
		return retryError
	}
	return nil
}

func convertGVR(gvr metav1.GroupVersionResource) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    gvr.Group,
		Version:  gvr.Version,
		Resource: gvr.Resource,
	}
}

func isChanOpen(ch chan struct{}) bool {
	open := true
	select {
	case _, open = <-ch:
	default:
	}
	return open
}

func (c *Controller) getResource(gvr *schema.GroupVersionResource) *configv1alpha1.Resource {
	for i := range c.RegisteredResources {
		if compareGvr(gvr, &c.RegisteredResources[i].GroupVersionResource) {
			return c.RegisteredResources[i].DeepCopy()
		}
	}
	return nil
}

func compareGvr(gvr1 *schema.GroupVersionResource, gvr2 *metav1.GroupVersionResource) bool {
	return gvr1.Group == gvr2.Group && gvr1.Resource == gvr2.Resource && gvr1.Version == gvr2.Version
}
