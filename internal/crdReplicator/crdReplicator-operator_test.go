package crdReplicator

import (
	"context"
	netv1alpha1 "github.com/liqoTech/liqo/api/net/v1alpha1"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"testing"
	"time"
)

var (
	remoteClusterID = "testRemoteClusterID"
	localClusterID  = "testLocalClusterID"
)

func getObj() *unstructured.Unstructured {
	networkConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "net.liqo.io/v1alpha1",
			"kind":       "NetworkConfig",
			"metadata": map[string]interface{}{
				"name":   "test-networkconfig",
				"labels": map[string]string{},
			},
			"spec": map[string]interface{}{
				"clusterID":       "clusterID-test",
				"podCIDR":         "10.0.0.0/12",
				"tunnelPublicIP":  "192.16.5.1",
				"tunnelPrivateIP": "192.168.4.1",
			},
		},
	}
	networkConfig.SetLabels(getLabels())
	return networkConfig
}

func getLabels() map[string]string {
	return map[string]string{
		LocalLabelSelector: "true",
		DestinationLabel:   remoteClusterID,
	}
}

func getCRDReplicator() CRDReplicatorReconciler {
	return CRDReplicatorReconciler{
		Scheme:                nil,
		ClusterID:             localClusterID,
		RemoteDynClients:      map[string]dynamic.Interface{remoteClusterID: dynClient},
		RegisteredResources:   nil,
		UnregisteredResources: nil,
		RemoteWatchers:        map[string]map[string]chan bool{},
		LocalDynClient:        dynClient,
		LocalWatchers:         map[string]map[string]chan bool{},
	}
}

func TestCRDReplicatorReconciler_CreateResource(t *testing.T) {
	networkConfig := getObj()
	d := getCRDReplicator()
	//test 1
	//the resource does not exist on the cluster
	//we expect to be created
	err := d.CreateResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	//test 2
	//the resource exists on the cluster and is the same
	//we expect not to be created and returns nil
	err = d.CreateResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	//test 3
	//the resource has different values than the existing one
	//we expect for the resource to be deleted and recreated
	networkConfig.SetLabels(map[string]string{"labelTestin": "test"})
	err = d.CreateResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	//test 4
	//the resource is not a valid one
	//we expect an error
	networkConfig.SetAPIVersion("invalidOne")
	networkConfig.SetName("newName")
	err = d.CreateResource(dynClient, gvr, networkConfig, clusterID)
	assert.NotNil(t, err, "error should not be nil")
	//test 5
	//the resource schema is not correct
	//we expect an error
	err = d.CreateResource(dynClient, schema.GroupVersionResource{}, networkConfig, clusterID)
	assert.NotNil(t, err, "error should not be nil")

}

func TestCRDReplicatorReconciler_DeleteResource(t *testing.T) {
	d := getCRDReplicator()
	//test 1
	//delete an existing resource
	//we expect the error to be nil
	networkConfig := getObj()
	err := d.CreateResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	err = d.DeleteResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	//test 2
	//deleting a resource that does not exist
	//we expect an error
	err = d.DeleteResource(dynClient, gvr, networkConfig, clusterID)
	assert.NotNil(t, err, "error should be not nil")
}

func TestCRDReplicatorReconciler_UpdateResource(t *testing.T) {
	d := getCRDReplicator()
	//first we create the resource
	networkConfig := getObj()
	err := d.CreateResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")

	//Test 1
	//we update the metadata section
	//we expect a nil error and the metadata section of the resource on the server to be equal
	networkConfig.SetLabels(map[string]string{"labelTesting": "test"})
	err = d.UpdateResource(dynClient, gvr, networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	obj, err := dynClient.Resource(gvr).Get(context.TODO(), networkConfig.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be nil")

	//Test 2
	//we update the spec section
	//we expect a nil error and the spec section of the resource on the server to be equal as we set it
	newSpec, err := getSpec(networkConfig, clusterID)
	assert.Nil(t, err, "error should be nil")
	newSpec["podCIDR"] = "1.1.1.1"
	//setting the new values of spec fields
	err = unstructured.SetNestedMap(obj.Object, newSpec, "spec")
	assert.Nil(t, err, "error should be nil")
	err = d.UpdateResource(dynClient, gvr, obj, clusterID)
	assert.Nil(t, err, "error should be nil")
	obj, err = dynClient.Resource(gvr).Get(context.TODO(), networkConfig.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be nil")
	spec, err := getSpec(obj, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, newSpec, spec, "specs should be equal")

	//Test 3
	//we update the status section
	//we expect a nil error and the status section of the resource on the server to be equal as we set it
	newStatus := map[string]interface{}{
		"natEnabled": true,
	}
	err = unstructured.SetNestedMap(obj.Object, newStatus, "status")
	assert.Nil(t, err, "error should be nil")
	err = d.UpdateResource(dynClient, gvr, obj, clusterID)
	assert.Nil(t, err, "error should be nil")
	obj, err = dynClient.Resource(gvr).Get(context.TODO(), networkConfig.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be nil")
	status, err := getStatus(obj, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, newStatus, status, "status should be equal")
}

func TestCRDReplicatorReconciler_StartWatchers(t *testing.T) {
	d := getCRDReplicator()
	//for each test we have a number of registered resources and
	//after calling the StartWatchers function we expect two have a certain number of active watchers
	//as is the number of the registered resources
	test1 := []schema.GroupVersionResource{{
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "networkconfigs",
	}, {
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "tunnelendpoints",
	}}
	test2 := []schema.GroupVersionResource{}
	test3 := []schema.GroupVersionResource{{
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "networkconfigs",
	}}
	tests := []struct {
		test             []schema.GroupVersionResource
		expectedWatchers int
	}{
		{test1, 2},
		{test2, 0},
		{test3, 1},
	}

	for _, test := range tests {
		d.RegisteredResources = test.test
		d.StartWatchers()
		assert.Equal(t, test.expectedWatchers, len(d.RemoteWatchers[remoteClusterID]), "it should be the same")
		assert.Equal(t, test.expectedWatchers, len(d.LocalWatchers[remoteClusterID]), "it should be the same")
		//stop the watchers

		for k, ch := range d.RemoteWatchers[remoteClusterID] {
			close(ch)
			delete(d.RemoteWatchers[remoteClusterID], k)
			time.Sleep(1 * time.Second)
		}
		for k, ch := range d.LocalWatchers[remoteClusterID] {
			close(ch)
			delete(d.LocalWatchers[remoteClusterID], k)
			time.Sleep(1 * time.Second)
		}
	}
	//test on a closed channel
	//we close a channel of a running watcher an expect that the function restarts the watcher
	//we add a new channel on runningWatchers
	d.RemoteWatchers[remoteClusterID][test3[0].String()] = make(chan bool)
	d.LocalWatchers[remoteClusterID][test3[0].String()] = make(chan bool)
	close(d.RemoteWatchers[remoteClusterID][test3[0].String()])
	close(d.LocalWatchers[remoteClusterID][test3[0].String()])
	time.Sleep(1 * time.Second)
	d.StartWatchers()
	select {
	case _, ok := <-d.RemoteWatchers[remoteClusterID][test3[0].String()]:
		assert.True(t, ok, "should be true")
	default:
	}
	select {
	case _, ok := <-d.LocalWatchers[remoteClusterID][test3[0].String()]:
		assert.True(t, ok, "should be true")
	default:
	}
	assert.NotPanics(t, func() { close(d.RemoteWatchers[remoteClusterID][test3[0].String()]) }, "should not panic")
	assert.NotPanics(t, func() { close(d.LocalWatchers[remoteClusterID][test3[0].String()]) }, "should not panic")
}

func TestCRDReplicatorReconciler_StopWatchers(t *testing.T) {
	d := getCRDReplicator()
	//we add two kind of resources to be watched
	//then unregister them and check that the watchers have been closed as well
	test1 := []schema.GroupVersionResource{{
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "networkconfigs",
	}, {
		Group:    netv1alpha1.GroupVersion.Group,
		Version:  netv1alpha1.GroupVersion.Version,
		Resource: "tunnelendpoints",
	}}
	d.RegisteredResources = test1
	d.StartWatchers()
	assert.Equal(t, 2, len(d.RemoteWatchers[remoteClusterID]), "it should be 2")
	assert.Equal(t, 2, len(d.LocalWatchers[remoteClusterID]), "it should be 2")
	for _, r := range test1 {
		d.UnregisteredResources = append(d.UnregisteredResources, r.String())
	}
	d.StopWatchers()
	assert.Equal(t, 0, len(d.RemoteWatchers[remoteClusterID]), "it should be 0")
	assert.Equal(t, 0, len(d.LocalWatchers[remoteClusterID]), "it should be 0")
	d.UnregisteredResources = []string{}
	//test 2
	//we close previously a channel of a watcher and then we add the resource to the unregistered list
	//we expect than it does not panic and only one watcher is still active
	d.RegisteredResources = test1
	d.StartWatchers()
	assert.Equal(t, 2, len(d.RemoteWatchers[remoteClusterID]), "it should be 2")
	d.UnregisteredResources = append(d.UnregisteredResources, d.RegisteredResources[0].String())
	assert.NotPanics(t, func() { close(d.RemoteWatchers[remoteClusterID][d.RegisteredResources[0].String()]) }, "should not panic")
	d.StopWatchers()
	assert.Equal(t, 1, len(d.RemoteWatchers[remoteClusterID]), "it should be 0")
	assert.Equal(t, 1, len(d.LocalWatchers[remoteClusterID]), "it should be 0")
}

func TestCRDReplicatorReconciler_AddedHandler(t *testing.T) {
	d := getCRDReplicator()
	//test 1
	//adding a resource kind that exists on the cluster
	//we expect the resource to be created
	test1 := getObj()
	d.AddedHandler(test1, gvr)
	time.Sleep(1 * time.Second)
	obj, err := dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.True(t, areEqual(test1, obj), "the two objects should be equal")
	//remove the resource
	err = dynClient.Resource(gvr).Delete(context.TODO(), test1.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "should be nil")

	//test 2
	//adding a resource kind that the api server does not know
	//we expect an error to be returned
	d.AddedHandler(test1, schema.GroupVersionResource{})
	obj, err = dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.NotNil(t, err, "error should be not nil")
	assert.Nil(t, obj, "the object retrieved should be nil")
}
func TestCRDReplicatorReconciler_ModifiedHandler(t *testing.T) {
	d := getCRDReplicator()

	//test 1
	//the modified resource does not exist on the cluster
	//we expect the resource to be created and error to be nil
	test1 := getObj()
	d.ModifiedHandler(test1, gvr)
	time.Sleep(1 * time.Second)
	obj, err := dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.True(t, areEqual(test1, obj), "the two objects should be equal")

	//test 2
	//the modified resource already exists on the cluster
	//we expect the resource to be modified and the error to be nil
	newSpec := map[string]interface{}{
		"clusterID":       "clusterID-test-modified",
		"podCIDR":         "10.0.0.0/12",
		"tunnelPublicIP":  "192.16.5.1",
		"tunnelPrivateIP": "192.168.4.1",
	}
	newStatus := map[string]interface{}{
		"podCIDRNAT": "10.200.0.0/12",
	}
	err = unstructured.SetNestedMap(obj.Object, newSpec, "spec")
	assert.Nil(t, err)
	err = unstructured.SetNestedMap(obj.Object, newStatus, "status")
	assert.Nil(t, err)
	obj.SetLabels(test1.GetLabels())
	d.ModifiedHandler(obj, gvr)
	time.Sleep(10 * time.Second)
	newObj, err := dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.True(t, areEqual(newObj, obj), "the two objects should be equal")
	//clean up the resource
	err = dynClient.Resource(gvr).Delete(context.TODO(), test1.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "should be nil")
}

func TestCRDReplicatorReconciler_RemoteResourceModifiedHandler(t *testing.T) {
	d := getCRDReplicator()

	//test 1
	//the modified resource does not exist on the cluster
	//we expect the resource to be created and error to be nil
	test1 := getObj()
	d.RemoteResourceModifiedHandler(test1, gvr, remoteClusterID)
	time.Sleep(1 * time.Second)
	_, err := dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "error should be not found")

	//test 2
	//the modified resource already exists on the cluster
	//we modify some fields other than status
	//we expect the resource to not be modified and the error to be nil
	test1, err = dynClient.Resource(gvr).Create(context.TODO(), test1, metav1.CreateOptions{})
	assert.Nil(t, err, "error should be nil")
	test1.SetLabels(map[string]string{
		"labelTestin": "labelling",
	})
	d.RemoteResourceModifiedHandler(test1, gvr, remoteClusterID)
	time.Sleep(1 * time.Second)
	obj, err := dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.NotEqual(t, obj.GetLabels(), test1.GetLabels(), "the labels of the two objects should be ")

	//clean up the resource
	err = dynClient.Resource(gvr).Delete(context.TODO(), test1.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "should be nil")
}

func TestCRDReplicatorReconciler_DeletedHandler(t *testing.T) {
	d := getCRDReplicator()
	//test 1
	//we create a resource then we pass it to the handler
	//we expect the resource to be deleted
	test1 := getObj()
	obj, err := dynClient.Resource(gvr).Create(context.TODO(), test1, metav1.CreateOptions{})
	assert.Nil(t, err, "error should be nil")
	assert.True(t, areEqual(test1, obj), "the two objects should be equal")
	d.DeletedHandler(obj, gvr)
	obj, err = dynClient.Resource(gvr).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.NotNil(t, err, "error should not be empty")
	assert.Nil(t, obj, "the object retrieved should be nil")
}

func TestIsOpen(t *testing.T) {
	//test 1
	//create a bool channel
	//expect to be opened
	ch := make(chan bool, 1)
	result := isOpen(ch)
	assert.True(t, result, "channel should be open")

	//test 2
	//write to the channel
	//expect to be opened
	ch <- true
	result = isOpen(ch)
	assert.True(t, result, "channel should be open")

	//test 3
	//close the channel and check if is closed
	//expect to be closed
	assert.NotPanics(t, func() { close(ch) }, "this should not panic, because the channel is opened")
	result = isOpen(ch)
	assert.False(t, result, "channel should be closed")

}

func TestGetSpec(t *testing.T) {
	spec := map[string]interface{}{
		"clusterID": "clusterID-test",
	}
	//test 1
	//we have an object with a spec field
	//we expect to get the spec and a nil error
	test1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": spec,
		},
	}
	objSpec, err := getSpec(test1, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, spec, objSpec, "the two specs should be equal")

	//test 2
	//we have an object without a spec field
	//we expect the error to be and a nil spec to be returned because the specied field is not found
	test2 := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	objSpec, err = getSpec(test2, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Nil(t, objSpec, "the spec should be nil")
}

func TestGetStatus(t *testing.T) {
	status := map[string]interface{}{
		"natEnabled": true,
	}
	//test 1
	//we have an object with a status field
	//we expect to get the status and a nil error
	test1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"status": status,
		},
	}
	objStatus, err := getStatus(test1, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, status, objStatus, "the two specs should be equal")

	//test 2
	//we have an object without a spec field
	//we expect the error to be and a nil spec to be returned because the specied field is not found
	test2 := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}
	objStatus, err = getStatus(test2, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Nil(t, objStatus, "the spec should be nil")
}
