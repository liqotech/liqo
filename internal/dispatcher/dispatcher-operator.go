package dispatcher

import (
	"context"
	"fmt"
	"github.com/liqoTech/liqo/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=foreignclusters/status,verbs=get;update;patch

type DispatcherReconciler struct {
	Scheme *runtime.Scheme
	//for each remote cluster we save dynamic client connected to its API server
	RemoteDynClients map[string]dynamic.Interface
	//dynamic client pointing to the local API server
	LocalDynClient dynamic.Interface
	//a list of GVRs of resources to be replicated
	RegisteredResources []schema.GroupVersionResource
	//each time a resource is removed from the configuration it is saved in this list,
	//it stays here until the associated watcher, if running, is stopped
	UnregisteredResources []string
	//for each cluster we save the tuples (registeredResource, chan)
	RunningWatchers map[string]chan bool
	Started         bool
}

func (d *DispatcherReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	d.StartWatchers()
	defer d.StopWatchers()

	return ctrl.Result{
		Requeue:      false,
		RequeueAfter: 5 * time.Second,
	}, nil
}

func (d *DispatcherReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ForeignCluster{}).
		Complete(d)
}

func (d *DispatcherReconciler) Watcher(gvr schema.GroupVersionResource, stop chan bool) {
	watcher, err := d.LocalDynClient.Resource(gvr).Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.Error(err, gvr)
		close(stop)
		klog.Infof("stopping the local watcher for resource: %s", gvr)
		return
	}
	event := watcher.ResultChan()
	for {
		select {
		case <-stop:
			klog.Infof("stopping the local watcher for resource: %s", gvr)
			return
		case e := <-event:
			obj, ok := e.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			switch e.Type {
			case watch.Added:
				klog.Infof("the resource %s %s has been added  on local cluster", obj.GetName(), gvr.String())
				d.AddedHandler(obj, gvr)
			case watch.Modified:
				klog.Infof("the resource %s %s has been modified on local cluster ", obj.GetName(), gvr.String())
				d.ModifiedHandler(obj, gvr)
			case watch.Deleted:
				klog.Infof("the resource %s %s has been deleted from local cluster ", obj.GetName(), gvr.String())
				d.DeletedHandler(obj, gvr)
			}
		}
	}
}

func (d *DispatcherReconciler) StartWatchers() {
	//for each resource check if there is already a running watcher
	for _, r := range d.RegisteredResources {
		//if there is not then start one
		if _, ok := d.RunningWatchers[r.String()]; !ok {
			stop := make(chan bool)
			d.RunningWatchers[r.String()] = stop
			go d.Watcher(r, stop)
			klog.Infof("starting watcher for resource: %s", r.String())
		} else {
			//here we check if the channel is closed and if so then we start again the watcher
			if !isOpen(d.RunningWatchers[r.String()]) {
				//it means that the channel is closed we need to restart the watcher
				stop := make(chan bool)
				d.RunningWatchers[r.String()] = stop
				go d.Watcher(r, stop)
				klog.Infof("starting watcher for resource: %s", r.String())
			}
		}
	}
}

//Stops all the watchers for the resources that have been unregistered
func (d *DispatcherReconciler) StopWatchers() {
	for _, r := range d.UnregisteredResources {
		if ch, ok := d.RunningWatchers[r]; ok {
			if !isOpen(ch) {
				//it means that the channel is closed we need only to delete the entry in the map
				delete(d.RunningWatchers, r)
			} else {
				close(ch)
				delete(d.RunningWatchers, r)
				klog.Infof("stopping watcher for resource: %s", r)
			}
		}
	}
}

func (d *DispatcherReconciler) CreateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	//check if the resource exists
	name := obj.GetName()
	namespace := obj.GetNamespace()
	klog.Infof("creating resource %s of type %s on cluster %s", name, gvr.String(), clusterID)
	r, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("an error occurred while getting resource %s of type %s on cluster %s: %s", name, gvr.String(), clusterID, err)
		return err
	}
	if found {
		//the resource already exists check if the resources are the same
		if areEqual(obj, r) {
			klog.Infof("resource %s of type %s already exists on cluster %s ", obj.GetName(), gvr.String(), clusterID)
			return nil
		} else {
			//if not equal delete the remote one
			err := client.Resource(gvr).Namespace(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("an error occurred while deleting resource %s on cluster %s: %s", name, clusterID, err)
				return err
			}
		}
	}
	//if we come here it means that we have to create the resource on the remote cluster
	spec, b, err := unstructured.NestedMap(obj.Object, "spec")
	if !b || err != nil {
		klog.Errorf("an error occurred while processing the 'spec' of the resource %s: %v ", name, err)
		return err
	}
	remRes := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": obj.GetAPIVersion(),
			"kind":       obj.GetKind(),
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels":    obj.GetLabels(),
			},
			"spec": spec,
		},
	}
	//create the resource on the remote cluster
	_, err = client.Resource(gvr).Create(context.TODO(), remRes, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("an error occurred while creating the resource %s of type %s on remote cluster %s: %s", name, gvr.String(), clusterID, err)
		return err
	}
	return err
}

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
	if !reflect.DeepEqual(local.GetLabels(), remote.GetLabels()) {
		return false
	}
	return true
}

func getSpec(obj *unstructured.Unstructured, clusterID string) (map[string]interface{}, error) {
	spec, b, err := unstructured.NestedMap(obj.Object, "spec")
	if !b {
		klog.Infof("spec field not found for resource %s %s of type %s on cluster %s", obj.GetName(), obj.GetNamespace(), obj.GetKind(), clusterID)
		return nil, nil
	}
	if err != nil {
		klog.Errorf("an error occurred while getting the spec field from resource %s %s of type %s on cluster %s: %s", obj.GetName(), obj.GetNamespace(), obj.GetKind(), clusterID, err)
		return nil, err
	}
	return spec, nil
}

func getStatus(obj *unstructured.Unstructured, clusterID string) (map[string]interface{}, error) {
	status, b, err := unstructured.NestedMap(obj.Object, "status")
	if !b {
		klog.Infof("status field not found for resource %s %s of type %s on cluster %s", obj.GetName(), obj.GetNamespace(), obj.GetKind(), clusterID)
		return nil, nil
	}
	if err != nil {
		klog.Errorf("an error occurred while getting the status field from resource %s %s of type %s on cluster %s: %s", obj.GetName(), obj.GetNamespace(), obj.GetKind(), clusterID, err)
		return nil, err
	}
	return status, nil
}

func (d *DispatcherReconciler) GetResource(client dynamic.Interface, gvr schema.GroupVersionResource, name, namespace, clusterID string) (*unstructured.Unstructured, bool, error) {
	r, err := client.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return r, true, nil
	} else if apierrors.IsNotFound(err) {
		return nil, false, nil
	} else {
		klog.Errorf("an error occurred while getting resource %s of type %s on cluster %s: %s", name, gvr.String(), clusterID, err)
		return nil, false, err
	}
}

func (d *DispatcherReconciler) AddedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	for cluster := range d.RemoteDynClients {
		err := d.CreateResource(d.RemoteDynClients[cluster], gvr, obj, cluster)
		if err != nil {
			klog.Error(err)
		}
	}
}

func (d *DispatcherReconciler) ModifiedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	for cluster := range d.RemoteDynClients {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		client := d.RemoteDynClients[cluster]
		clusterID := cluster
		//we check if the resource exists in the remote cluster
		_, found, err := d.GetResource(client, gvr, name, namespace, cluster)
		if err != nil {
			klog.Errorf("an error occurred while getting resource %s of type %s on cluster %s: %s", name, gvr.String(), clusterID, err)
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
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return d.UpdateResource(client, gvr, obj, clusterID)
		})
		if retryErr != nil {
			klog.Error(retryErr)
		}
	}
}

func (d *DispatcherReconciler) DeletedHandler(obj *unstructured.Unstructured, gvr schema.GroupVersionResource) {
	for cluster := range d.RemoteDynClients {
		name := obj.GetName()
		namespace := obj.GetNamespace()
		client := d.RemoteDynClients[cluster]
		clusterID := cluster
		//we check if the resource exists in the remote cluster
		_, found, err := d.GetResource(client, gvr, name, namespace, cluster)
		if err != nil {
			klog.Errorf("an error occurred while getting resource %s of type %s on cluster %s: %s", name, gvr.String(), clusterID, err)
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

func (d *DispatcherReconciler) UpdateResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	name := obj.GetName()
	namespace := obj.GetNamespace()
	klog.Infof("updating resource %s of type %s on remote cluster %s", name, gvr.String(), clusterID)
	// Retrieve the latest version of resource before attempting update
	r, found, err := d.GetResource(client, gvr, name, namespace, clusterID)
	if err != nil {
		klog.Errorf("an error occurred while getting resource %s of type %s on cluster %s: %s", name, gvr.String(), clusterID, err)
		return err
	}
	//this one should never happen, if it does then someone deleted the resource on the other cluster
	if !found {
		klog.Errorf("an error occurred while getting resource %s of type %s on cluster %s: %s", name, gvr.String(), clusterID, err)
		return fmt.Errorf("something strange happened, check if the resource %s of type %s on cluster %s exists on the remote cluster", name, gvr.String(), clusterID)
	}
	//check if metadata has to be updated
	if !reflect.DeepEqual(obj.GetLabels(), r.GetLabels()) {
		newMetaData := metav1.ObjectMeta{
			Labels: obj.GetLabels(),
		}
		klog.Infof("updating metadata of resource %s of type %s on remote cluster %s", name, gvr.String(), clusterID)
		if err := d.UpdateMetadata(client, gvr, r, clusterID, newMetaData); err != nil {
			return err
		}
	}
	//get spec fields for both resources
	localSpec, err := getSpec(obj, "localCluster")
	if err != nil {
		return err
	}
	remoteSpec, err := getSpec(r, clusterID)
	if err != nil {
		return err
	}
	//check if are the same and if not update the remote one
	if !reflect.DeepEqual(localSpec, remoteSpec) {
		klog.Infof("updating spec field of resource %s of type %s on remote cluster %s", name, gvr.String(), clusterID)
		err := d.UpdateSpec(client, gvr, r, clusterID, localSpec)
		if err != nil {
			return err
		}
	}
	//get status field for both resources
	localStatus, err := getStatus(obj, "localCluster")
	if err != nil {
		return err
	}
	remoteStatus, err := getStatus(r, clusterID)
	if err != nil {
		return err
	}
	//check if are the same and if not update the remote one
	if !reflect.DeepEqual(localStatus, remoteStatus) {
		klog.Infof("updating status field resource %s of type %s on remote cluster %s", name, gvr.String(), clusterID)
		err := d.UpdateStatus(client, gvr, r, clusterID, localStatus)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DispatcherReconciler) DeleteResource(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string) error {
	klog.Infof("deleting resource %s of type %s on cluster %s", obj.GetName(), gvr.String(), clusterID)
	err := client.Resource(gvr).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("an error occurred while deleting resource %s of type %s on cluster %s: %s", obj.GetName(), gvr.String(), clusterID, err)
		return err
	}
	return nil
}

//updates the following field of metadata: name, namespace, labels
func (d *DispatcherReconciler) UpdateMetadata(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, m metav1.ObjectMeta) error {
	//setting the new values of metadata
	obj.SetLabels(m.Labels)
	//update the remote resource
	_, err := client.Resource(gvr).Update(context.TODO(), obj, metav1.UpdateOptions{})
	if err != nil && apierrors.IsConflict(err) {
		klog.Errorf("an error occurred while updating metadata field of resource %s of type %s on cluster %s: %s", m.Name, gvr.String(), clusterID, err)
	}
	return err
}

//updates the spec field of a resource
func (d *DispatcherReconciler) UpdateSpec(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, spec map[string]interface{}) error {
	//setting the new values of spec fields
	err := unstructured.SetNestedMap(obj.Object, spec, "spec")
	if err != nil {
		klog.Errorf("an error occurred while setting the new spec fields of resource %s of kind %s for cluster %s: %s", obj.GetName(), gvr.String(), clusterID, err)
		return err
	}
	//update the remote resource
	_, err = client.Resource(gvr).Update(context.TODO(), obj, metav1.UpdateOptions{})
	if err != nil && !apierrors.IsConflict(err) {
		klog.Errorf("an error occurred while updating spec field of resource %s of type %s on cluster %s: %s", obj.GetName(), gvr.String(), clusterID, err)
	}
	return err
}

//updates the status field of a resource
func (d *DispatcherReconciler) UpdateStatus(client dynamic.Interface, gvr schema.GroupVersionResource, obj *unstructured.Unstructured, clusterID string, status map[string]interface{}) error {
	//setting the new values of spec fields
	err := unstructured.SetNestedMap(obj.Object, status, "status")
	if err != nil {
		klog.Errorf("an error occurred while setting the new status fields of resource %s of kind %s for cluster %s: %s", obj.GetName(), gvr.String(), clusterID, err)
		return err
	}
	//update the remote resource
	_, err = client.Resource(gvr).UpdateStatus(context.TODO(), obj, metav1.UpdateOptions{})
	if err != nil && !apierrors.IsConflict(err) {
		klog.Errorf("an error occurred while updating status field of resource %s of type %s on cluster %s: %s", obj.GetName(), gvr.String(), clusterID, err)
	}
	return err
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
