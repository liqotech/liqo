package crdreplicator

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	utils "github.com/liqotech/liqo/pkg/liqonet"
	tenantcontrolnamespace "github.com/liqotech/liqo/pkg/tenantControlNamespace"
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
	RegisteredResources            []schema.GroupVersionResource                           // a list of GVRs of resources to be replicated
	UnregisteredResources          []string                                                // each time a resource is removed from the configuration it is saved in this list, it stays here until the associated watcher, if running, is stopped
	LocalWatchers                  map[string]chan struct{}                                // we save all the running watchers monitoring the local resources:(registeredResource, chan))
	RemoteWatchers                 map[string]map[string]chan struct{}                     // for each peering cluster we save all the running watchers monitoring the replicated resources:(clusterID, (registeredResource, chan))

	UseNewAuth                   bool
	NamespaceManager             tenantcontrolnamespace.TenantControlNamespaceManager
	IdentityManager              identitymanager.IdentityManager
	LocalToRemoteNamespaceMapper map[string]string
	RemoteToLocalNamespaceMapper map[string]string
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

// Reconcile handles requests for subscribed types of object.
func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var fc v1alpha1.ForeignCluster
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
		if !fc.Status.Incoming.Joined && !fc.Status.Outgoing.Joined {
			if utils.ContainsString(fc.ObjectMeta.Finalizers, finalizer) {
				fc.Finalizers = utils.RemoveString(fc.Finalizers, finalizer)
				if err := c.Update(ctx, &fc); err != nil {
					klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", fc.Name, err)
					return result, err
				}
				return result, nil
			}
			return result, nil
		}
		if !utils.ContainsString(fc.ObjectMeta.Finalizers, finalizer) {
			fc.ObjectMeta.Finalizers = append(fc.Finalizers, finalizer)
			if err := c.Update(ctx, &fc); err != nil {
				klog.Errorf("%s -> unable to update resource %s: %s", c.ClusterID, fc.Name, err)
				return result, err
			}
		}
	} else {
		// the object is being deleted
		if utils.ContainsString(fc.Finalizers, finalizer) {
			// close remote watcher for remote cluster
			rWatchers, ok := c.RemoteWatchers[remoteClusterID]
			if ok {
				for r, ch := range rWatchers {
					klog.Infof("%s -> closing remote watcher for resource %s", remoteClusterID, r)
					close(ch)
				}
				delete(c.RemoteWatchers, remoteClusterID)
			}

			// delete dynamic client for remote cluster
			delete(c.RemoteDynClients, remoteClusterID)
			// delete informer for remote cluster
			delete(c.RemoteDynSharedInformerFactory, remoteClusterID)
			// remove the finalizer from the list and update it.
			fc.Finalizers = utils.RemoveString(fc.Finalizers, finalizer)
			if err := c.Update(ctx, &fc); err != nil {
				klog.Errorf("an error occurred while updating resource %s after the finalizer has been removed: %s", fc.Name, err)
				return result, err
			}
			return result, nil
		}
	}
	// check if the client already exists
	// check if the dynamic dynamic client and informer factory exists
	_, dynClientOk := c.RemoteDynClients[remoteClusterID]
	_, dynFacOk := c.RemoteDynSharedInformerFactory[remoteClusterID]
	if dynClientOk && dynFacOk {
		return result, nil
	}

	if c.UseNewAuth {
		if fc.Status.TenantControlNamespace.Local == "" || fc.Status.TenantControlNamespace.Remote == "" {
			klog.V(4).Infof("%s -> tenantControlNamespace is not set in resource %s for remote peering cluster %s",
				c.ClusterID, req.NamespacedName, remoteClusterID)
			return result, nil
		}
		config, err := c.IdentityManager.GetConfig(remoteClusterID, fc.Status.TenantControlNamespace.Local)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s",
				c.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, nil
		}
		return result, c.setUpConnectionToPeeringCluster(config, remoteClusterID, &fc)
	}

	// check if the config of the peering cluster is ready
	// first we check the outgoing connection
	if fc.Status.Outgoing.AvailableIdentity {
		// retrieve the config
		config, err := c.getKubeConfig(c.ClientSet, fc.Status.Outgoing.IdentityRef, remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s", c.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, nil
		}
		return result, c.setUpConnectionToPeeringCluster(config, remoteClusterID, &fc)
	} else if fc.Status.Incoming.AvailableIdentity {
		// retrieve the config
		config, err := c.getKubeConfig(c.ClientSet, fc.Status.Incoming.IdentityRef, remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s", c.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, err
		}
		return result, c.setUpConnectionToPeeringCluster(config, remoteClusterID, &fc)
	}
	return result, nil
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	resourceToBeProccesedPredicate := predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).Named(operatorName).WithEventFilter(resourceToBeProccesedPredicate).
		For(&v1alpha1.ForeignCluster{}).
		Complete(c)
}

func (c *Controller) getKubeConfig(clientset kubernetes.Interface, reference *corev1.ObjectReference, remoteClusterID string) (*rest.Config, error) {
	if reference == nil {
		return nil, fmt.Errorf("%s -> object reference for the secret containing kubeconfig of foreign cluster %s not set yet", c.ClusterID, remoteClusterID)
	}
	secret, err := clientset.CoreV1().Secrets(reference.Namespace).Get(context.TODO(), reference.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	kubeconfig := func() (*clientcmdapi.Config, error) {
		return clientcmd.Load(secret.Data["kubeconfig"])
	}
	cnf, err := clientcmd.BuildConfigFromKubeconfigGetter("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return cnf, nil
}

func (c *Controller) setUpConnectionToPeeringCluster(config *rest.Config, remoteClusterID string, fc *v1alpha1.ForeignCluster) error {
	var remoteNamespace string
	if c.UseNewAuth {
		c.LocalToRemoteNamespaceMapper[fc.Status.TenantControlNamespace.Local] = fc.Status.TenantControlNamespace.Remote
		c.RemoteToLocalNamespaceMapper[fc.Status.TenantControlNamespace.Remote] = fc.Status.TenantControlNamespace.Local
		remoteNamespace = fc.Status.TenantControlNamespace.Remote
	} else {
		remoteNamespace = metav1.NamespaceAll
	}

	// check if the dynamic dynamic client exists
	if _, ok := c.RemoteDynClients[remoteClusterID]; !ok {
		dynClient, err := dynamic.NewForConfig(config)
		if err != nil {
			klog.Errorf("%s -> unable to create dynamic client in order to create the dynamic shared informer factory: %s", remoteClusterID, err)
			// we don't need to immediately requeue the foreign cluster but wait for the next re-sync
			return nil
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
	return nil
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

func (c *Controller) remoteModifiedWrapper(oldObj, newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := c.getGVR(objUnstruct)
	remoteClusterID := objUnstruct.GetLabels()[DestinationLabel]
	c.RemoteResourceModifiedHandler(objUnstruct, gvr, remoteClusterID)
}

func (c *Controller) RemoteResourceModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource, remoteClusterId string) {
	name := obj.GetName()
	namespace := obj.GetNamespace()

	localNamespace := c.remoteToLocalNamespace(namespace)

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
		klog.Infof("%s -> removing resource %s in namespace %s of type %s", remoteClusterId, name, localNamespace, gvr.String())
		err := c.DeleteResource(c.RemoteDynClients[remoteClusterId], gvr, obj, remoteClusterId)
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
	localStatus, err := getStatus(localObj, remoteClusterId)
	if err != nil {
		return
	}
	// check if are the same and if not update the local one
	if !reflect.DeepEqual(localStatus, remoteStatus) {
		klog.Infof("%s -> updating status field of resource %s of type %s", clusterID, name, gvr.String())
		_ = c.UpdateStatus(localDynClient, gvr, localObj, clusterID, remoteStatus)
		return
	}
}

func (c *Controller) StartWatchers() {
	// for each remote cluster check if the remote watchers are running for each registered resource
	for remCluster, remDynFac := range c.RemoteDynSharedInformerFactory {
		watchers := c.RemoteWatchers[remCluster]
		if watchers == nil {
			watchers = make(map[string]chan struct{})
		}
		for _, res := range c.RegisteredResources {
			// if there is not then start one
			if _, ok := watchers[res.String()]; !ok {
				stopCh := make(chan struct{})
				watchers[res.String()] = stopCh
				go c.Watcher(remDynFac, res, cache.ResourceEventHandlerFuncs{
					UpdateFunc: c.remoteModifiedWrapper,
				}, stopCh)
				klog.Infof("%s -> starting remote watcher for resource: %s", remCluster, res.String())
			}
		}
		c.RemoteWatchers[remCluster] = watchers
	}
	// check if the local watchers are running for each registered resource
	for _, res := range c.RegisteredResources {
		// if there is not a running local watcher then start one
		if _, ok := c.LocalWatchers[res.String()]; !ok {
			stopCh := make(chan struct{})
			c.LocalWatchers[res.String()] = stopCh
			go c.Watcher(c.LocalDynSharedInformerFactory, res, cache.ResourceEventHandlerFuncs{
				AddFunc:    c.AddFunc,
				UpdateFunc: c.UpdateFunc,
				DeleteFunc: c.DeleteFunc,
			}, stopCh)
			klog.Infof("%s -> starting local watcher for resource: %s", c.ClusterID, res.String())
		}
	}
}

// Stops all the watchers for the resources that have been unregistered.
func (c *Controller) StopWatchers() {
	// stop all remote watchers for unregistered resources
	for remCluster, watchers := range c.RemoteWatchers {
		for _, res := range c.UnregisteredResources {
			if ch, ok := watchers[res]; ok {
				if ok {
					close(ch)
					delete(watchers, res)
					klog.Infof("%s -> stopping remote watcher for resource: %s", remCluster, res)
				}
			}
		}
		c.RemoteWatchers[remCluster] = watchers
	}
	// stop all local watchers
	for _, res := range c.UnregisteredResources {
		if ch, ok := c.LocalWatchers[res]; ok {
			if ok {
				close(ch)
				delete(c.LocalWatchers, res)
				klog.Infof("%s -> stopping local watcher for resource: %s", c.ClusterID, res)
			}
		}
	}
}

func (c *Controller) CreateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	// check if the resource exists
	name := obj.GetName()
	namespace := obj.GetNamespace()

	klog.Infof("%s -> creating resource %s of type %s", clusterID, name, gvr.String())
	r, found, err := c.GetResource(client, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return err
	}
	if found {
		// the resource already exists check if the resources are the same
		if areEqual(obj, r) {
			klog.Infof("%s -> resource %s of type %s already exists", clusterID, obj.GetName(), gvr.String())
			return nil
		} else {
			// if not equal delete the remote one
			err := client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
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
	_, err = client.Resource(gvr).Namespace(namespace).Create(context.TODO(), remRes, metav1.CreateOptions{})
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

// checks if the spec and status of two resources are the same.
func areEqual(local, remote *unstructured.Unstructured) bool {
	localSpec, b, err := unstructured.NestedMap(local.Object, "spec")
	if !b || err != nil {
		return false
	}
	remoteSpec, b, err := unstructured.NestedMap(remote.Object, "spec")
	if !b || err != nil {
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

	specEqual := reflect.DeepEqual(localSpec, remoteSpec)
	statusEqual := reflect.DeepEqual(localStatus, remoteStatus)
	if !specEqual || !statusEqual {
		return false
	}
	return true
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
	c.AddedHandler(objUnstruct, gvr)
}

func (c *Controller) AddedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	// check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", c.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}

	remoteNamespace := c.localToRemoteNamespace(obj.GetNamespace())
	obj.SetNamespace(remoteNamespace)

	if dynClient, ok := c.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", c.ClusterID, remoteClusterID)
		return
	} else {
		err := c.CreateResource(dynClient, gvr, obj, remoteClusterID)
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
	gvr := c.getGVR(objUnstruct)
	c.ModifiedHandler(objUnstruct, gvr)
}

func (c *Controller) ModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
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
			err := c.CreateResource(dynClient, gvr, obj, clusterID)
			if err != nil {
				klog.Error(err)
			}
		}
		// if the resource exists or we just created it then we update the fields
		// we do this considering that the resource existed, even if we just created it
		if err = c.UpdateResource(dynClient, gvr, obj, clusterID); err != nil {
			klog.Errorf("%s -> an error occurred while updating resource %s of type %s: %s", clusterID, name, gvr.String(), err)
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
	c.DeletedHandler(objUnstruct, gvr)
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

func (c *Controller) UpdateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	// Retrieve the latest version of resource before attempting update
	r, found, err := c.GetResource(client, gvr, name, namespace, clusterID)
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
		err := c.UpdateSpec(client, gvr, r, clusterID, localSpec)
		if err != nil {
			return err
		}
	}
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
		err := c.UpdateStatus(client, gvr, r, clusterID, localStatus)
		if err != nil {
			return err
		}
	}
	return nil
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
