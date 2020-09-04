package crdReplicator

import (
	"context"
	"fmt"
	"github.com/liqoTech/liqo/api/discovery/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

var (
	LocalLabelSelector     = "liqo.io/replication"
	RemoteLabelSelector    = "liqo.io/originID"
	DestinationLabel       = "liqo.io/remoteID"
	ReplicationStatuslabel = "liqo.io/replicated"
	result                 = ctrl.Result{
		RequeueAfter: 5 * time.Second,
	}
)

type CRDReplicatorReconciler struct {
	Scheme *runtime.Scheme
	client.Client
	ClientSet *kubernetes.Clientset
	ClusterID string
	//for each remote cluster we save dynamic client connected to its API server
	RemoteDynClients map[string]dynamic.Interface
	//dynamic client pointing to the local API server
	LocalDynClient dynamic.Interface
	//a list of GVRs of resources to be replicated
	RegisteredResources []schema.GroupVersionResource
	//each time a resource is removed from the configuration it is saved in this list,
	//it stays here until the associated watcher, if running, is stopped
	UnregisteredResources []string
	//for each peering cluster we save all the running watchers monitoring the local resources:(clusterID, (registeredResource, chan))
	LocalWatchers map[string]map[string]chan bool
	//for each peering cluster we save all the running watchers monitoring the replicated resources:(clusterID, (registeredResource, chan))
	RemoteWatchers map[string]map[string]chan bool
}

func (d *CRDReplicatorReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	var fc v1alpha1.ForeignCluster
	ctx := context.Background()

	err := d.Get(ctx, req.NamespacedName, &fc)
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("%s -> unable to retrieve resource %s: %s", d.ClusterID, req.NamespacedName, err)
		return result, err
	}
	if apierrors.IsNotFound(err) {
		klog.Errorf("%s -> resource %s not present, probably deleted: %s", d.ClusterID, req.NamespacedName, err)
		return ctrl.Result{}, nil
	}
	remoteClusterID := fc.Spec.ClusterID
	//check if the config of the peering cluster is ready
	//first we check the outgoing connection
	if fc.Status.Outgoing.AvailableIdentity {
		//retrieve the config
		config, err := d.getKubeConfig(d.ClientSet, fc.Status.Outgoing.IdentityRef, remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s", d.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, nil
		}
		//check if the dynamic client exists
		if _, ok := d.RemoteDynClients[remoteClusterID]; !ok {
			dynClient, err := dynamic.NewForConfig(config)
			if err != nil {
				klog.Errorf("%s -> unable to create dynamic client: %s", remoteClusterID, err)
				return result, err
			} else {
				klog.Infof("%s -> dynamic client created", remoteClusterID)
			}
			d.RemoteDynClients[remoteClusterID] = dynClient
		}
	} else if fc.Status.Incoming.AvailableIdentity {
		//retrieve the config
		config, err := d.getKubeConfig(d.ClientSet, fc.Status.Incoming.IdentityRef, remoteClusterID)
		if err != nil {
			klog.Errorf("%s -> unable to retrieve config from resource %s for remote peering cluster %s: %s", d.ClusterID, req.NamespacedName, remoteClusterID, err)
			return result, err
		}
		//check if the dynamic client exists
		if _, ok := d.RemoteDynClients[remoteClusterID]; !ok {
			dynClient, err := dynamic.NewForConfig(config)
			if err != nil {
				klog.Errorf("%s -> unable to create dynamic client: %s", remoteClusterID, err)
				return result, err
			} else {
				klog.Infof("%s -> dynamic client created", remoteClusterID)
			}
			d.RemoteDynClients[remoteClusterID] = dynClient
		}
	}

	d.StartWatchers()
	d.StopWatchers()
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

func (d *CRDReplicatorReconciler) LocalWatcher(gvr schema.GroupVersionResource, stop chan bool, remoteClusterID string) {
	watcher, err := d.LocalDynClient.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: LocalLabelSelector + "=true" + "," + DestinationLabel + "=" + remoteClusterID,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while starting local watcher for resource %s :%s", remoteClusterID, gvr.String(), err)
		//closing the channel, so at the next reconcile the watcher will be restarted
		close(stop)
		return
	}
	event := watcher.ResultChan()
	for {
		select {
		case <-stop:
			klog.Infof("%s -> stopping the local watcher for resource: %s", remoteClusterID, gvr)
			return
		case e := <-event:
			obj, ok := e.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			switch e.Type {
			case watch.Added:
				klog.Infof("%s -> the local resource %s %s has been added", remoteClusterID, obj.GetName(), gvr.String())
				d.AddedHandler(obj, gvr)
			case watch.Modified:
				klog.Infof("%s -> the local resource %s %s has been modified", remoteClusterID, obj.GetName(), gvr.String())
				d.ModifiedHandler(obj, gvr)
			case watch.Deleted:
				klog.Infof("%s -> the local resource %s %s has been deleted", remoteClusterID, obj.GetName(), gvr.String())
				d.DeletedHandler(obj, gvr)
			}
		}
	}
}

func (d *CRDReplicatorReconciler) RemoteWatcher(dynClient dynamic.Interface, gvr schema.GroupVersionResource, stop chan bool, remoteClusterID string) {
	watcher, err := dynClient.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{
		LabelSelector: RemoteLabelSelector + "=" + d.ClusterID,
	})
	if err != nil {
		klog.Errorf("%s -> an error occurred while starting local watcher for resource %s :%s", remoteClusterID, gvr.String(), err)
		//closing the channel, so at the next reconcile the watcher will be restarted
		close(stop)
		return
	}
	event := watcher.ResultChan()
	for {
		select {
		case <-stop:
			klog.Infof("%s -> stopping the remote watcher for resource: %s", remoteClusterID, gvr)
			return
		case e := <-event:
			obj, ok := e.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			switch e.Type {
			case watch.Modified:
				klog.Infof("%s -> the remote resource %s %s has been modified ", remoteClusterID, obj.GetName(), gvr.String())
				d.RemoteResourceModifiedHandler(obj, gvr, remoteClusterID)
			}
		}
	}
}

func (d *CRDReplicatorReconciler) RemoteResourceModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource, remoteClusterId string) {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	client := d.LocalDynClient
	clusterID := d.ClusterID
	//we check if the resource exists in the local cluster
	localObj, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
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
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		//we reflect on the local resource only the changes of the status of the remote one
		//we do not reflect the changes to labels, annotations or spec
		//TODO:support labels and annotations

		//get status field for both resources
		remoteStatus, err := getStatus(obj, clusterID)
		if err != nil {
			return err
		}
		localStatus, err := getStatus(localObj, remoteClusterId)
		if err != nil {
			return err
		}
		//check if are the same and if not update the local one
		if !reflect.DeepEqual(localStatus, remoteStatus) {
			klog.Infof("%s -> updating status field of resource %s of type %s", clusterID, name, gvr.String())
			err := d.UpdateStatus(client, gvr, localObj, clusterID, remoteStatus)
			return err
		}
		return nil
	})
	if retryErr != nil {
		klog.Error(retryErr)
	}
}

func (d *CRDReplicatorReconciler) StartWatchers() {
	//for each remote cluster check if the remote watchers are running for each registered resource
	for remCluster, remDynClient := range d.RemoteDynClients {
		watchers := d.RemoteWatchers[remCluster]
		if watchers == nil {
			watchers = make(map[string]chan bool)
		}
		for _, res := range d.RegisteredResources {
			//if there is not then start one
			if _, ok := watchers[res.String()]; !ok {
				stop := make(chan bool, 1)
				watchers[res.String()] = stop
				go d.RemoteWatcher(remDynClient, res, stop, remCluster)
				klog.Infof("%s -> starting remote watcher for resource: %s", remCluster, res.String())
			} else {
				//here we check if the channel is closed and if so then we start again the watcher
				if !isOpen(watchers[res.String()]) {
					//it means that the channel is closed we need to restart the watcher
					stop := make(chan bool, 1)
					watchers[res.String()] = stop
					go d.RemoteWatcher(remDynClient, res, stop, remCluster)
					klog.Infof("%s -> starting remote watcher for resource: %s", remCluster, res.String())
				}
			}
		}
		d.RemoteWatchers[remCluster] = watchers
	}
	//for each remote cluster check if the local watchers are running for each registered resource
	for remCluster := range d.RemoteDynClients {
		watchers := d.LocalWatchers[remCluster]
		if watchers == nil {
			watchers = make(map[string]chan bool)
		}
		for _, res := range d.RegisteredResources {
			//if there is not a running local watcher then start one
			if _, ok := watchers[res.String()]; !ok {
				stop := make(chan bool, 1)
				watchers[res.String()] = stop
				go d.LocalWatcher(res, stop, remCluster)
				klog.Infof("%s -> starting local watcher for resource: %s", remCluster, res.String())
			} else {
				//here we check if the channel is closed and if so then we start again the watcher
				if !isOpen(watchers[res.String()]) {
					//it means that the channel is closed we need to restart the watcher
					stop := make(chan bool, 1)
					watchers[res.String()] = stop
					go d.LocalWatcher(res, stop, remCluster)
					klog.Infof("%s -> starting local watcher for resource: %s", remCluster, res.String())
				}
			}
		}
		d.LocalWatchers[remCluster] = watchers
	}
}

//Stops all the watchers for the resources that have been unregistered
func (d *CRDReplicatorReconciler) StopWatchers() {
	//stop all remote watchers
	for remCluster, watchers := range d.RemoteWatchers {
		for _, res := range d.UnregisteredResources {
			if ch, ok := watchers[res]; ok {
				if !isOpen(ch) {
					//it means that the channel is closed we need only to delete the entry in the map
					delete(watchers, res)
				} else {
					close(ch)
					delete(watchers, res)
				}
			}
		}
		d.RemoteWatchers[remCluster] = watchers
	}
	//stop all local watchers
	for remCluster, watchers := range d.LocalWatchers {
		for _, res := range d.UnregisteredResources {
			if ch, ok := watchers[res]; ok {
				if !isOpen(ch) {
					//it means that the channel is closed we need only to delete the entry in the map
					delete(watchers, res)
				} else {
					close(ch)
					delete(watchers, res)
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
	//deleting the clusterID of the peering cluster where we are gonna replicate the custom resource
	delete(labels, DestinationLabel)
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
		client := dynClient
		clusterID := remoteClusterID
		//we check if the resource exists in the remote cluster
		_, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
		//if the resource does not exist then we create it
		if !found {
			err := d.CreateResource(client, gvr, obj, clusterID)
			if err != nil {
				klog.Error(err)
			}
		}
		//if the resource exists or we just created it then we update the fields
		//we do this considering that the resource existed, even if we just created it
		if err = d.UpdateResource(client, gvr, obj, clusterID); err != nil {
			klog.Errorf("%s -> an error occurred while updating resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
	}
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
		client := dynClient
		clusterID := remoteClusterID
		//we check if the resource exists in the remote cluster
		_, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
		if err != nil {
			klog.Errorf("%s -> an error occurred while getting resource %s of type %s: %s", clusterID, name, gvr.String(), err)
			return
		}
		//if the resource exists on the remote cluster then we delete it
		if found {
			err := d.DeleteResource(client, gvr, obj, clusterID)
			if err != nil {
				klog.Error(err)
			}
		}
	}
}

func (d *CRDReplicatorReconciler) UpdateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	klog.Infof("%s -> updating resource %s of type %s", clusterID, name, gvr.String())
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
	//check if metadata has to be updated
	if !reflect.DeepEqual(obj.GetLabels(), r.GetLabels()) {
		newMetaData := metav1.ObjectMeta{
			Labels: d.UpdateLabels(obj.GetLabels()),
		}
		klog.Infof("%s -> updating metadata of resource %s of type %s", clusterID, name, gvr.String())
		if err := d.UpdateMetadata(client, gvr, r, clusterID, newMetaData); err != nil {
			return err
		}
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

//updates the following field of metadata: labels
func (d *CRDReplicatorReconciler) UpdateMetadata(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, m metav1.ObjectMeta) error {
	//setting the new values of metadata
	obj.SetLabels(m.Labels)
	//update the remote resource
	_, err := client.Resource(gvr).Namespace(obj.GetNamespace()).Update(context.TODO(), obj, metav1.UpdateOptions{})
	if err != nil && apierrors.IsConflict(err) {
		klog.Errorf("%s -> an error occurred while updating metadata field of resource %s %s of type %s: %s", clusterID, obj.GetName(), obj.GetNamespace(), gvr.String(), err)
	}
	return err
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
		//setting the new values of spec fields
		err = unstructured.SetNestedMap(res.Object, status, "status")
		if err != nil {
			klog.Errorf("%s -> an error occurred while setting the new status fields of resource %s %s of kind %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), err)
			return err
		}
		//update the remote resource
		_, err = client.Resource(gvr).Namespace(obj.GetNamespace()).UpdateStatus(context.TODO(), res, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("%s -> an error occurred while updating status field of resource %s %s of type %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), err)
			return err
		}
		return nil
	})
	if retryError != nil {
		klog.Errorf("%s -> an error occurred while updating status field of resource %s %s of type %s: %s", clusterID, res.GetName(), res.GetNamespace(), gvr.String(), retryError)
		return retryError
	}
	return nil
}

func isOpen(ch chan bool) bool {
	select {
	case _, ok := <-ch:
		if ok {
			//it means that the channel is open so we return true
			return true
		} else {
			return false
		}
	default:
		//if it returns but no values are ready we return true so it means that is open
		return true
	}
}
