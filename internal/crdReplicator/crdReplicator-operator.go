package crdReplicator

import (
	"context"
	"fmt"
	"github.com/liqotech/liqo/api/discovery/v1alpha1"
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
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

var (
	ResyncPeriod           = 30 * time.Second
	LocalLabelSelector     = "liqo.io/replication"
	RemoteLabelSelector    = "liqo.io/originID"
	DestinationLabel       = "liqo.io/remoteID"
	ReplicationStatuslabel = "liqo.io/replicated"
	result                 = ctrl.Result{
		RequeueAfter: 30 * time.Second,
	}
)

type CRDReplicatorReconciler struct {
	Scheme *runtime.Scheme
	client.Client
	ClientSet *kubernetes.Clientset
	ClusterID string
	//for each remote cluster we save dynamic client connected to its API server
	RemoteDynClients map[string]dynamic.Interface
	//for each remote cluster we save the dynamic shared informer factory
	RemoteDynSharedInformerFactory map[string]dynamicinformer.DynamicSharedInformerFactory
	//dynamic client pointing to the local API server
	LocalDynClient dynamic.Interface
	//local dynamic shared informer factory
	LocalDynSharedInformerFactory dynamicinformer.DynamicSharedInformerFactory
	//a list of GVRs of resources to be replicated
	RegisteredResources []schema.GroupVersionResource
	//each time a resource is removed from the configuration it is saved in this list,
	//it stays here until the associated watcher, if running, is stopped
	UnregisteredResources []string
	//for each peering cluster we save all the running watchers monitoring the local resources:(clusterID, (registeredResource, chan))
	LocalWatchers map[string]map[string]chan struct{}
	//for each peering cluster we save all the running watchers monitoring the replicated resources:(clusterID, (registeredResource, chan))
	RemoteWatchers map[string]map[string]chan struct{}
}

func (d *CRDReplicatorReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var fc v1alpha1.ForeignCluster
	ctx := context.Background()
	defer d.StartWatchers()
	defer d.StopWatchers()
	err := d.Get(ctx, req.NamespacedName, &fc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("%s -> unable to retrieve resource %s: %s", d.ClusterID, req.NamespacedName, err)
		return result, err
	}
	if apierrors.IsNotFound(err) {
		klog.Errorf("%s -> resource %s not present, probably deleted: %s", d.ClusterID, req.NamespacedName, err)
		return ctrl.Result{}, nil
	}
	remoteClusterID := fc.Spec.ClusterIdentity.ClusterID
	//check if the client already exists
	//check if the dynamic dynamic client and informer factory exists
	_, dynClientOk := d.RemoteDynClients[remoteClusterID]
	_, dynFacOk := d.RemoteDynSharedInformerFactory[remoteClusterID]
	if dynClientOk && dynFacOk {
		return result, nil
	}
	//check if the config of the peering cluster is ready
	//first we check the outgoing connection
	if fc.Status.Outgoing.AvailableIdentity {
		//retrieve the config
		config, err := d.getKubeConfig(d.ClientSet, fc.Status.Outgoing.IdentityRef, remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s", d.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, nil
		}
		return result, d.setUpConnectionToPeeringCluster(config, remoteClusterID)

	} else if fc.Status.Incoming.AvailableIdentity {
		//retrieve the config
		config, err := d.getKubeConfig(d.ClientSet, fc.Status.Incoming.IdentityRef, remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s", d.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, err
		}
		return result, d.setUpConnectionToPeeringCluster(config, remoteClusterID)
	}
	return result, nil
}

func (d *CRDReplicatorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ForeignCluster{}).
		Complete(d)
}

func (d *CRDReplicatorReconciler) getKubeConfig(clientset kubernetes.Interface, reference *corev1.ObjectReference, remoteClusterID string) (*rest.Config, error) {
	if reference == nil {
		return nil, fmt.Errorf("%s -> object reference for the secret containing kubeconfig of foreign cluster %s not set yet", d.ClusterID, remoteClusterID)
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

func (d *CRDReplicatorReconciler) setUpConnectionToPeeringCluster(config *rest.Config, remoteClusterID string) error {
	//check if the dynamic dynamic client exists
	if _, ok := d.RemoteDynClients[remoteClusterID]; !ok {
		dynClient, err := dynamic.NewForConfig(config)
		if err != nil {
			klog.Errorf("%s -> unable to create dynamic client in order to create the dynamic shared informer factory: %s", remoteClusterID, err)
			//we don't need to immediately requeue the foreign cluster but wait for the next re-sync
			return nil
		} else {
			klog.Infof("%s -> dynamic client created", remoteClusterID)
		}
		d.RemoteDynClients[remoteClusterID] = dynClient
	}
	//check if the dynamic shared informer factory exists
	if _, ok := d.RemoteDynSharedInformerFactory[remoteClusterID]; !ok {
		f := dynamicinformer.NewFilteredDynamicSharedInformerFactory(d.RemoteDynClients[remoteClusterID], ResyncPeriod, metav1.NamespaceAll, d.SetLabelsForRemoteResources)
		d.RemoteDynSharedInformerFactory[remoteClusterID] = f
		klog.Infof("%s -> dynamic shared informer factory created", remoteClusterID)
	}
	return nil
}

func (d *CRDReplicatorReconciler) SetLabelsForRemoteResources(options *metav1.ListOptions) {
	//we want to watch only the resources that have been created by us on the remote cluster
	if options.LabelSelector == "" {
		newLabelSelector := []string{RemoteLabelSelector, "=", d.ClusterID}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	} else {
		newLabelSelector := []string{options.LabelSelector, RemoteLabelSelector, "=", d.ClusterID}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}

func SetLabelsForLocalResources(options *metav1.ListOptions) {
	//we want to watch only the resources that should be replicated on a remote cluster
	if options.LabelSelector == "" {
		newLabelSelector := []string{LocalLabelSelector, "=", "true"}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	} else {
		newLabelSelector := []string{options.LabelSelector, LocalLabelSelector, "=", "true"}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}

func (d *CRDReplicatorReconciler) Watcher(dynFac dynamicinformer.DynamicSharedInformerFactory, gvr schema.GroupVersionResource, handlerFuncs cache.ResourceEventHandlerFuncs, stopCh chan struct{}) {
	//get informer for resource
	inf := dynFac.ForResource(gvr)
	inf.Informer().AddEventHandler(handlerFuncs)
	inf.Informer().Run(stopCh)
}

func (d *CRDReplicatorReconciler) getGVR(obj *unstructured.Unstructured) schema.GroupVersionResource {
	gvk := obj.GroupVersionKind()
	resuource := strings.ToLower(gvk.Kind)
	gvr := schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: resuource + "s",
	}
	return gvr
}

func (d *CRDReplicatorReconciler) remoteModifiedWrapper(oldObj, newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := d.getGVR(objUnstruct)
	remoteClusterID := objUnstruct.GetLabels()[DestinationLabel]
	d.RemoteResourceModifiedHandler(objUnstruct, gvr, remoteClusterID)
}

func (d *CRDReplicatorReconciler) RemoteResourceModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource, remoteClusterId string) {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	localDynClient := d.LocalDynClient
	clusterID := d.ClusterID
	//we check if the resource exists in the local cluster
	localObj, found, err := d.GetResource(localDynClient, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return
	}
	// TODO if the resource does not exist what do we do?
	//do nothing? remove the remote replication?
	//current choice -> remove the remote one as well
	if !found {
		klog.Infof("%s -> resource %s in namespace %s of type %s not found", clusterID, name, namespace, gvr.String())
		klog.Infof("%s -> removing resource %s in namespace %s of type %s", remoteClusterId, name, namespace, gvr.String())
		err := d.DeleteResource(d.RemoteDynClients[remoteClusterId], gvr, obj, remoteClusterId)
		if err != nil {
			return
		}
		return
	}
	//if the resource exists on the local cluster then we update the status

	//we reflect on the local resource only the changes of the status of the remote one
	//we do not reflect the changes to labels, annotations or spec
	//TODO:support labels and annotations

	//get status field for both resources
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
	//check if are the same and if not update the local one
	if !reflect.DeepEqual(localStatus, remoteStatus) {
		klog.Infof("%s -> updating status field of resource %s of type %s", clusterID, name, gvr.String())
		_ = d.UpdateStatus(localDynClient, gvr, localObj, clusterID, remoteStatus)
		return
	}
}

func (d *CRDReplicatorReconciler) StartWatchers() {
	//for each remote cluster check if the remote watchers are running for each registered resource
	for remCluster, remDynFac := range d.RemoteDynSharedInformerFactory {
		watchers := d.RemoteWatchers[remCluster]
		if watchers == nil {
			watchers = make(map[string]chan struct{})
		}
		for _, res := range d.RegisteredResources {
			//if there is not then start one
			if _, ok := watchers[res.String()]; !ok {
				stopCh := make(chan struct{})
				watchers[res.String()] = stopCh
				go d.Watcher(remDynFac, res, cache.ResourceEventHandlerFuncs{
					UpdateFunc: d.remoteModifiedWrapper,
				}, stopCh)
				klog.Infof("%s -> starting remote watcher for resource: %s", remCluster, res.String())
			}
		}
		d.RemoteWatchers[remCluster] = watchers
	}
	//for each remote cluster check if the local watchers are running for each registered resource
	for remCluster := range d.RemoteDynClients {
		watchers := d.LocalWatchers[remCluster]
		if watchers == nil {
			watchers = make(map[string]chan struct{})
		}
		for _, res := range d.RegisteredResources {
			//if there is not a running local watcher then start one
			if _, ok := watchers[res.String()]; !ok {
				stopCh := make(chan struct{})
				watchers[res.String()] = stopCh
				go d.Watcher(d.LocalDynSharedInformerFactory, res, cache.ResourceEventHandlerFuncs{
					AddFunc:    d.AddFunc,
					UpdateFunc: d.UpdateFunc,
					DeleteFunc: d.DeleteFunc,
				}, stopCh)
				klog.Infof("%s -> starting local watcher for resource: %s", remCluster, res.String())
			}
		}
		d.LocalWatchers[remCluster] = watchers
	}
}

//Stops all the watchers for the resources that have been unregistered
func (d *CRDReplicatorReconciler) StopWatchers() {
	//stop all remote watchers for unregistered resources
	for remCluster, watchers := range d.RemoteWatchers {
		for _, res := range d.UnregisteredResources {
			if ch, ok := watchers[res]; ok {
				if ok {
					close(ch)
					delete(watchers, res)
					klog.Infof("%s -> stopping remote watcher for resource: %s", remCluster, res)
				}
			}

		}
		d.RemoteWatchers[remCluster] = watchers
	}
	//stop all local watchers
	for remCluster, watchers := range d.LocalWatchers {
		for _, res := range d.UnregisteredResources {
			if ch, ok := watchers[res]; ok {
				if ok {
					close(ch)
					delete(watchers, res)
					klog.Infof("%s -> stopping local watcher for resource: %s", remCluster, res)
				}
			}
		}
		d.LocalWatchers[remCluster] = watchers
	}

}

func (d *CRDReplicatorReconciler) CreateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	//check if the resource exists
	name := obj.GetName()
	namespace := obj.GetNamespace()
	klog.Infof("%s -> creating resource %s of type %s", clusterID, name, gvr.String())
	r, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return err
	}
	if found {
		//the resource already exists check if the resources are the same
		if areEqual(obj, r) {
			klog.Infof("%s -> resource %s of type %s already exists", clusterID, obj.GetName(), gvr.String())
			return nil
		} else {
			//if not equal delete the remote one
			err := client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("%s -> an error occurred while deleting resource %s: %s", clusterID, name, err)
				return err
			}
		}
	}
	//if we come here it means that we have to create the resource on the remote cluster
	spec, b, err := unstructured.NestedMap(obj.Object, "spec")
	if !b || err != nil {
		klog.Errorf("%s -> an error occurred while processing the 'spec' of the resource %s: %s ", d.ClusterID, name, err)
		return err
	}
	remRes := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": obj.GetAPIVersion(),
			"kind":       obj.GetKind(),
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    d.UpdateLabels(obj.GetLabels()),
			},
			"spec": spec,
		},
	}
	//create the resource on the remote cluster
	_, err = client.Resource(gvr).Namespace(namespace).Create(context.TODO(), remRes, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("%s -> an error occurred while creating the resource %s %s of type %s: %s", clusterID, name, namespace, gvr.String(), err)
		return err
	}
	return err
}

func (d *CRDReplicatorReconciler) UpdateLabels(labels map[string]string) map[string]string {
	//we don't check if the map is nil, because it has to be initialized because we use the label to filter the resources
	//which needs to be replicated
	//setting the replication label to false
	labels[LocalLabelSelector] = "false"
	//setting replication status to true
	labels[ReplicationStatuslabel] = "true"
	//setting originID i.e clusterID of home cluster
	labels[RemoteLabelSelector] = d.ClusterID
	return labels
}

//checks if the spec and status of two resources are the same
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

func (d *CRDReplicatorReconciler) GetResource(client dynamic.Interface, gvr schema.GroupVersionResource, name, namespace, clusterID string) (*unstructured.Unstructured, bool, error) {
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

func (d *CRDReplicatorReconciler) AddFunc(newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := d.getGVR(objUnstruct)
	d.AddedHandler(objUnstruct, gvr)
}

func (d *CRDReplicatorReconciler) AddedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	//check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", d.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}
	if dynClient, ok := d.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", d.ClusterID, remoteClusterID)
		return
	} else {
		err := d.CreateResource(dynClient, gvr, obj, remoteClusterID)
		if err != nil {
			klog.Error(err)
		}
	}
}

func (d *CRDReplicatorReconciler) UpdateFunc(oldObj, newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := d.getGVR(objUnstruct)
	d.ModifiedHandler(objUnstruct, gvr)
}

func (d *CRDReplicatorReconciler) ModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	//check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", d.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}

	if dynClient, ok := d.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", d.ClusterID, remoteClusterID)
		return
	} else {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		clusterID := remoteClusterID
		//we check if the resource exists in the remote cluster
		_, found, err := d.GetResource(dynClient, gvr, name, namespace, clusterID)
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
		//if the resource does not exist then we create it
		if !found {
			err := d.CreateResource(dynClient, gvr, obj, clusterID)
			if err != nil {
				klog.Error(err)
			}
		}
		//if the resource exists or we just created it then we update the fields
		//we do this considering that the resource existed, even if we just created it
		if err = d.UpdateResource(dynClient, gvr, obj, clusterID); err != nil {
			klog.Errorf("%s -> an error occurred while updating resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
	}
}

func (d *CRDReplicatorReconciler) DeleteFunc(newObj interface{}) {
	objUnstruct, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement newObj to unstructured object")
		return
	}
	gvr := d.getGVR(objUnstruct)
	d.DeletedHandler(objUnstruct, gvr)
}

func (d *CRDReplicatorReconciler) DeletedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	//check if already exists a cluster to the remote peering cluster specified in the labels
	labels := obj.GetLabels()
	remoteClusterID, ok := labels[DestinationLabel]
	if !ok {
		klog.Infof("%s -> resource %s %s of type %s has not a destination label with the ID of the peering cluster", d.ClusterID, obj.GetName(), obj.GetNamespace(), gvr.String())
		return
	}

	if dynClient, ok := d.RemoteDynClients[remoteClusterID]; !ok {
		klog.Infof("%s -> a connection to the peering cluster with id: %s does not exist", d.ClusterID, remoteClusterID)
		return
	} else {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		dynClient := dynClient
		clusterID := remoteClusterID
		//we check if the resource exists in the remote cluster
		_, found, err := d.GetResource(dynClient, gvr, name, namespace, clusterID)
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
		//if the resource exists on the remote cluster then we delete it
		if found {
			err := d.DeleteResource(dynClient, gvr, obj, clusterID)
			if err != nil {
				klog.Error(err)
			}
		}
	}
}

func (d *CRDReplicatorReconciler) UpdateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	// Retrieve the latest version of resource before attempting update
	r, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return err
	}
	//this one should never happen, if it does then someone deleted the resource on the other cluster
	if !found {
		klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
		return fmt.Errorf("something strange happened, check if the resource %s of type %s on cluster %s exists on the remote cluster", name, gvr.String(), clusterID)
	}
	//get spec fields for both resources
	localSpec, err := getSpec(obj, d.ClusterID)
	if err != nil {
		return err
	}
	remoteSpec, err := getSpec(r, clusterID)
	if err != nil {
		return err
	}
	//check if are the same and if not update the remote one
	if !reflect.DeepEqual(localSpec, remoteSpec) {
		klog.Infof("%s -> updating spec field of resource %s of type %s", clusterID, name, gvr.String())
		err := d.UpdateSpec(client, gvr, r, clusterID, localSpec)
		if err != nil {
			return err
		}
	}
	//get status field for both resources
	localStatus, err := getStatus(obj, d.ClusterID)
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
	//check if are the same and if not update the remote one
	if !reflect.DeepEqual(localStatus, remoteStatus) {
		klog.Infof("%s -> updating status field resource %s of type %s", clusterID, name, gvr.String())
		err := d.UpdateStatus(client, gvr, r, clusterID, localStatus)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *CRDReplicatorReconciler) DeleteResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	klog.Infof("%s -> deleting resource %s of type %s", clusterID, obj.GetName(), gvr.String())
	err := client.Resource(gvr).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("%s -> an error occurred while deleting resource %s of type %s: %s", clusterID, obj.GetName(), gvr.String(), err)
		return err
	}
	return nil
}

//updates the spec field of a resource
func (d *CRDReplicatorReconciler) UpdateSpec(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, spec map[string]interface{}) error {
	res := &unstructured.Unstructured{}
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		//get the latest version of the resource before attempting to update it
		res, err := client.Resource(gvr).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting the latest version of resource %s %s of kind %s before attempting to update its spec field: %s", clusterID, obj.GetName(), obj.GetNamespace(), gvr.String(), err)
			return err
		}
		//setting the new values of spec fields
		err = unstructured.SetNestedMap(res.Object, spec, "spec")
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting the new spec fields of resource %s %s of kind %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), err)
			return err
		}
		//update the remote resource
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

//updates the status field of a resource
func (d *CRDReplicatorReconciler) UpdateStatus(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, status map[string]interface{}) error {
	res := &unstructured.Unstructured{}
	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		//get the latest version of the resource before attempting to update it
		res, err := client.Resource(gvr).Namespace(obj.GetNamespace()).Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting the latest version of resource %s %s of kind %s before attempting to update its status field: %s", clusterID, obj.GetName(), obj.GetNamespace(), gvr.String(), err)
			return err
		}
		//setting the new values of status fields
		err = unstructured.SetNestedMap(res.Object, status, "status")
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting the new status fields of resource %s %s of kind %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), err)
			return err
		}
		//update the remote resource
		_, err = client.Resource(gvr).Namespace(obj.GetNamespace()).UpdateStatus(context.TODO(), res, metav1.UpdateOptions{})
		return err
	})
	if retryError != nil {
		klog.Errorf("%s -> an error occurred while updating status field of resource %s %s of type %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), retryError)
		return retryError
	}
	return nil
}
