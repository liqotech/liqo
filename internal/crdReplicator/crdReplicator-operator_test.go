package crdreplicator

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
)

const (
	remoteClusterID = "testRemoteClusterID"
	localClusterID  = "testLocalClusterID"
	testNamespace   = "default"
)

func getObj() *unstructured.Unstructured {
	networkConfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "net.liqo.io/v1alpha1",
			"kind":       "NetworkConfig",
			"metadata": map[string]interface{}{
				"name":      "test-networkconfig",
				"namespace": testNamespace,
				"labels":    map[string]string{},
			},
			"spec": map[string]interface{}{
				"clusterID":      "clusterID-test",
				"podCIDR":        "10.0.0.0/12",
				"externalCIDR":   "192.168.0.0/24",
				"endpointIP":     "192.16.5.1",
				"backendType":    "wireguard",
				"backend_config": map[string]interface{}{},
			},
		},
	}
	networkConfig.SetLabels(getLabels())
	return networkConfig
}

func getObjNamespaced() *unstructured.Unstructured {
	resourceRequest := &discoveryv1alpha1.ResourceRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceRequest",
			APIVersion: discoveryv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "resourcerequest",
			Namespace: testNamespace,
		},
		Spec: discoveryv1alpha1.ResourceRequestSpec{
			AuthURL: "https://example.com",
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: "id",
			},
		},
	}
	resourceRequest.SetLabels(getLabels())
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resourceRequest)
	if err != nil {
		klog.Error(err)
		os.Exit(1)
	}
	return &unstructured.Unstructured{
		Object: obj,
	}
}

func getLabels() map[string]string {
	return map[string]string{
		LocalLabelSelector: "true",
		DestinationLabel:   remoteClusterID,
	}
}

func getCRDReplicator() Controller {
	tenantmanager := tenantnamespace.NewTenantNamespaceManager(k8sclient)
	clusterIDInterface := clusterid.NewStaticClusterID(localClusterID)
	return Controller{
		Scheme:                nil,
		ClusterID:             localClusterID,
		RemoteDynClients:      map[string]dynamic.Interface{remoteClusterID: dynClient},
		RegisteredResources:   nil,
		UnregisteredResources: nil,
		RemoteWatchers:        map[string]map[string]chan struct{}{},
		LocalDynClient:        dynClient,
		LocalWatchers:         map[string]chan struct{}{},

		NamespaceManager:                 tenantmanager,
		IdentityReader:                   identitymanager.NewCertificateIdentityReader(k8sclient, clusterIDInterface, tenantmanager),
		LocalToRemoteNamespaceMapper:     map[string]string{},
		RemoteToLocalNamespaceMapper:     map[string]string{},
		ClusterIDToLocalNamespaceMapper:  map[string]string{},
		ClusterIDToRemoteNamespaceMapper: map[string]string{},
	}
}

func setupReplication(d *Controller, ownership consts.OwnershipType) {
	d.ClusterIDToLocalNamespaceMapper["testRemoteClusterID"] = testNamespace
	d.RegisteredResources = []configv1alpha1.Resource{
		{
			GroupVersionResource: metav1.GroupVersionResource(netv1alpha1.NetworkConfigGroupVersionResource),
			PeeringPhase:         consts.PeeringPhaseAll,
			Ownership:            ownership,
		},
	}
}

func TestCRDReplicatorReconciler_CreateResource(t *testing.T) {
	networkConfig := getObj()
	d := getCRDReplicator()
	//test 1
	//the resource does not exist on the cluster
	//we expect to be created
	err := d.CreateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipShared)
	assert.Nil(t, err, "error should be nil")
	//test 2
	//the resource exists on the cluster and is the same
	//we expect not to be created and returns nil
	err = d.CreateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipShared)
	assert.Nil(t, err, "error should be nil")
	//test 3
	//the resource has different values than the existing one
	//we expect for the resource to be deleted and recreated
	networkConfig.SetLabels(map[string]string{"labelTestin": "test"})
	err = d.CreateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipShared)
	assert.Nil(t, err, "error should be nil")
	//test 4
	//the resource is not a valid one
	//we expect an error
	networkConfig.SetAPIVersion("invalidOne")
	networkConfig.SetName("newName")
	err = d.CreateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipShared)
	assert.NotNil(t, err, "error should not be nil")
	//test 5
	//the resource schema is not correct
	//we expect an error
	err = d.CreateResource(dynClient, schema.GroupVersionResource{}, networkConfig, clusterID, consts.OwnershipShared)
	assert.NotNil(t, err, "error should not be nil")

}

func TestCRDReplicatorReconciler_DeleteResource(t *testing.T) {
	d := getCRDReplicator()
	//test 1
	//delete an existing resource
	//we expect the error to be nil
	networkConfig := getObj()
	err := d.CreateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipShared)
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
	err := d.CreateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipLocal)
	assert.Nil(t, err, "error should be nil")

	//Test 1
	//we update the metadata section
	//we expect a nil error and the metadata section of the resource on the server to be equal
	networkConfig.SetLabels(map[string]string{"labelTesting": "test"})
	err = d.UpdateResource(dynClient, gvr, networkConfig, clusterID, consts.OwnershipLocal)
	assert.Nil(t, err, "error should be nil")
	obj, err := dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), networkConfig.GetName(), metav1.GetOptions{})
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
	err = d.UpdateResource(dynClient, gvr, obj, clusterID, consts.OwnershipLocal)
	assert.Nil(t, err, "error should be nil")
	obj, err = dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), networkConfig.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be nil")
	spec, err := getSpec(obj, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, newSpec, spec, "specs should be equal")

	//Test 3
	//we update the status section
	//we expect a nil error and the status section of the resource on the server to be equal as we set it
	newStatus := map[string]interface{}{
		"processed": true,
	}
	err = unstructured.SetNestedMap(obj.Object, newStatus, "status")
	assert.Nil(t, err, "error should be nil")
	err = d.UpdateResource(dynClient, gvr, obj, clusterID, consts.OwnershipLocal)
	assert.Nil(t, err, "error should be nil")
	obj, err = dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), networkConfig.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be nil")
	status, err := getStatus(obj, clusterID)
	assert.Nil(t, err, "error should be nil")
	assert.Equal(t, newStatus, status, "status should be equal")
}

func TestCRDReplicatorReconciler_StartAndStopWatchers(t *testing.T) {
	d := getCRDReplicator()
	d.setPeeringPhase(remoteClusterID, consts.PeeringPhaseOutgoing)
	d.ClusterIDToRemoteNamespaceMapper[remoteClusterID] = testNamespace
	//we add two kind of resources to be watched
	//then unregister them and check that the watchers have been closed as well
	test1 := []configv1alpha1.Resource{{
		GroupVersionResource: metav1.GroupVersionResource{
			Group:    netv1alpha1.GroupVersion.Group,
			Version:  netv1alpha1.GroupVersion.Version,
			Resource: "networkconfigs",
		},
		PeeringPhase: consts.PeeringPhaseEstablished,
		Ownership:    consts.OwnershipShared,
	}, {
		GroupVersionResource: metav1.GroupVersionResource{
			Group:    netv1alpha1.GroupVersion.Group,
			Version:  netv1alpha1.GroupVersion.Version,
			Resource: "tunnelendpoints",
		},
		PeeringPhase: consts.PeeringPhaseIncoming, // this will not be replicated with the current peering phase
		Ownership:    consts.OwnershipShared,
	}, {
		GroupVersionResource: metav1.GroupVersionResource{
			Group:    discoveryv1alpha1.GroupVersion.Group,
			Version:  discoveryv1alpha1.GroupVersion.Version,
			Resource: "resourcerequests",
		},
		PeeringPhase: consts.PeeringPhaseEstablished,
		Ownership:    consts.OwnershipShared,
	}}
	d.RegisteredResources = test1

	// create a fake replicated resource
	resourceRequest := &discoveryv1alpha1.ResourceRequest{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceRequest",
			APIVersion: discoveryv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: testNamespace,
			Labels: map[string]string{
				LocalLabelSelector:     "false",
				ReplicationStatuslabel: "true",
				RemoteLabelSelector:    d.ClusterID,
			},
		},
		Spec: discoveryv1alpha1.ResourceRequestSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: d.ClusterID,
			},
			AuthURL: "test",
		},
	}
	unstruct, err := runtime.DefaultUnstructuredConverter.ToUnstructured(resourceRequest)
	assert.Nil(t, err, "should be nil")
	_, err = d.RemoteDynClients[remoteClusterID].
		Resource(discoveryv1alpha1.GroupVersion.WithResource("resourcerequests")).
		Namespace(testNamespace).
		Create(context.TODO(), &unstructured.Unstructured{Object: unstruct}, metav1.CreateOptions{})
	assert.Nil(t, err, "should be nil")

	d.StartWatchers()
	assert.Equal(t, 2, len(d.RemoteWatchers[remoteClusterID]), "it should be 2")
	assert.Equal(t, 3, len(d.LocalWatchers), "it should be 3")
	for _, r := range test1 {
		d.UnregisteredResources = append(d.UnregisteredResources, r.GroupVersionResource)
	}
	d.StopWatchers()
	assert.Equal(t, 0, len(d.RemoteWatchers[remoteClusterID]), "it should be 0")
	d.UnregisteredResources = []metav1.GroupVersionResource{}

	// check that the resource has been deleted when the watcher stops
	list, err := d.RemoteDynClients[remoteClusterID].
		Resource(discoveryv1alpha1.GroupVersion.WithResource("resourcerequests")).
		List(context.TODO(), metav1.ListOptions{})
	assert.Nil(t, err, "should be nil")
	assert.Equal(t, 0, len(list.Items), "the list should be empty")
}

func TestCRDReplicatorReconciler_AddedHandler(t *testing.T) {
	d := getCRDReplicator()
	setupReplication(&d, consts.OwnershipShared)

	//test 1
	//adding a resource kind that exists on the cluster
	//we expect the resource to be created
	test1 := getObj()
	d.AddedHandler(test1, gvr)
	time.Sleep(1 * time.Second)
	obj, err := dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.True(t, areEqual(test1, obj), "the two objects should be equal")
	//remove the resource
	err = dynClient.Resource(gvr).Namespace(testNamespace).Delete(context.TODO(), test1.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "should be nil")

	//test 2
	//adding a resource kind that the api server does not know
	//we expect an error to be returned
	d.AddedHandler(test1, schema.GroupVersionResource{})
	obj, err = dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.NotNil(t, err, "error should be not nil")
	assert.Nil(t, obj, "the object retrieved should be nil")
}
func TestCRDReplicatorReconciler_ModifiedHandler(t *testing.T) {
	d := getCRDReplicator()
	setupReplication(&d, consts.OwnershipLocal)

	//test 1
	//the modified resource does not exist on the cluster
	//we expect the resource to be created and error to be nil
	test1 := getObj()
	d.ModifiedHandler(test1, gvr)
	time.Sleep(1 * time.Second)
	obj, err := dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.True(t, areEqual(test1, obj), "the two objects should be equal")

	//test 2
	//the modified resource already exists on the cluster
	//we expect the resource to be modified and the error to be nil
	newSpec := map[string]interface{}{
		"clusterID":      "clusterid-test-modified",
		"podCIDR":        "10.0.0.0/12",
		"externalCIDR":   "192.168.0.0/24",
		"endpointIP":     "192.16.5.1",
		"backendType":    "wireguard",
		"backend_config": map[string]interface{}{},
	}
	newStatus := map[string]interface{}{
		"podCIDRNAT":      "10.200.0.0/12",
		"externalCIDRNAT": "None",
		"processed":       true,
	}
	err = unstructured.SetNestedMap(obj.Object, newSpec, "spec")
	assert.Nil(t, err)
	err = unstructured.SetNestedMap(obj.Object, newStatus, "status")
	assert.Nil(t, err)
	obj.SetLabels(test1.GetLabels())
	d.ModifiedHandler(obj, gvr)
	time.Sleep(1 * time.Second)
	newObj, err := dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.True(t, areEqual(newObj, obj), "the two objects should be equal")
	//clean up the resource
	err = dynClient.Resource(gvr).Namespace(testNamespace).Delete(context.TODO(), test1.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "should be nil")
}

func TestCRDReplicatorReconciler_RemoteResourceModifiedHandler(t *testing.T) {
	d := getCRDReplicator()
	setupReplication(&d, consts.OwnershipShared)

	//test 1
	//the modified resource does not exist on the cluster
	//we expect the resource to be created and error to be nil
	test1 := getObj()
	d.RemoteResourceModifiedHandler(dynClient, test1, gvr, remoteClusterID, consts.OwnershipShared)
	time.Sleep(1 * time.Second)
	_, err := dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "error should be not found")

	//test 2
	//the modified resource already exists on the cluster
	//we modify some fields other than status
	//we expect the resource to not be modified and the error to be nil
	test1, err = dynClient.Resource(gvr).Namespace(testNamespace).Create(context.TODO(), test1, metav1.CreateOptions{})
	assert.Nil(t, err, "error should be nil")
	test1.SetLabels(map[string]string{
		"labelTestin": "labelling",
	})
	d.RemoteResourceModifiedHandler(dynClient, test1, gvr, remoteClusterID, consts.OwnershipShared)
	time.Sleep(1 * time.Second)
	obj, err := dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.NotEqual(t, obj.GetLabels(), test1.GetLabels(), "the labels of the two objects should be ")

	// test 3
	// the modified resource already exists on the cluster
	// we modify some fields in the status
	// we expect the resource to be modified and the error to be nil
	test1, err = dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be nil")
	newStatus := map[string]interface{}{
		"processed": true,
	}
	err = unstructured.SetNestedMap(obj.Object, newStatus, "status")
	assert.Nil(t, err, "error should be nil")
	d.RemoteResourceModifiedHandler(dynClient, test1, gvr, remoteClusterID, consts.OwnershipShared)
	time.Sleep(1 * time.Second)
	obj, err = dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.Nil(t, err, "error should be empty")
	assert.Equal(t, obj, test1, "the two objects should be equal")

	//clean up the resource
	err = dynClient.Resource(gvr).Namespace(testNamespace).Delete(context.TODO(), test1.GetName(), metav1.DeleteOptions{})
	assert.Nil(t, err, "should be nil")
}

func TestCRDReplicatorReconciler_DeletedHandler(t *testing.T) {
	d := getCRDReplicator()
	//test 1
	//we create a resource then we pass it to the handler
	//we expect the resource to be deleted
	test1 := getObj()
	obj, err := dynClient.Resource(gvr).Namespace(testNamespace).Create(context.TODO(), test1, metav1.CreateOptions{})
	assert.Nil(t, err, "error should be nil")
	assert.True(t, areEqual(test1, obj), "the two objects should be equal")
	d.DeletedHandler(obj, gvr)
	obj, err = dynClient.Resource(gvr).Namespace(testNamespace).Get(context.TODO(), test1.GetName(), metav1.GetOptions{})
	assert.NotNil(t, err, "error should not be empty")
	assert.Nil(t, obj, "the object retrieved should be nil")
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
		"processed": true,
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

func TestNamespaceTranslation(t *testing.T) {
	d := getCRDReplicator()

	remoteClusterID := "cluster-id"
	localNamespace := "local"
	remoteNamespace := "remote"
	otherNamespace := "other"

	d.LocalToRemoteNamespaceMapper[localNamespace] = remoteNamespace
	d.RemoteToLocalNamespaceMapper[remoteNamespaceKeyer(remoteClusterID, remoteNamespace)] = localNamespace

	// namespaces present in the map

	mappedNamespace := d.localToRemoteNamespace(localNamespace)
	assert.Equal(t, mappedNamespace, remoteNamespace, "these namespace names have to be equal")

	demappedNamespace := d.remoteToLocalNamespace(remoteClusterID, mappedNamespace)
	assert.Equal(t, demappedNamespace, localNamespace, "these namespace names have to be equal")

	// namespaces not present in the map

	mappedNamespace = d.localToRemoteNamespace(otherNamespace)
	assert.Equal(t, mappedNamespace, otherNamespace, "these namespace names have to be equal")

	demappedNamespace = d.remoteToLocalNamespace(remoteClusterID, mappedNamespace)
	assert.Equal(t, demappedNamespace, otherNamespace, "these namespace names have to be equal")
}

func TestNamespaceTranslationMultipleClusters(t *testing.T) {
	d := getCRDReplicator()

	fc1 := &discoveryv1alpha1.ForeignCluster{
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: "cluster-1",
			},
		},
		Status: discoveryv1alpha1.ForeignClusterStatus{
			TenantNamespace: discoveryv1alpha1.TenantNamespaceType{
				Local:  "local-1",
				Remote: "remote",
			},
		},
	}

	fc2 := &discoveryv1alpha1.ForeignCluster{
		Spec: discoveryv1alpha1.ForeignClusterSpec{
			ClusterIdentity: discoveryv1alpha1.ClusterIdentity{
				ClusterID: "cluster-2",
			},
		},
		Status: discoveryv1alpha1.ForeignClusterStatus{
			TenantNamespace: discoveryv1alpha1.TenantNamespaceType{
				Local:  "local-2",
				Remote: "remote", // the remote namespaces can have the same name
			},
		},
	}

	d.setUpTranslations(fc1)
	d.setUpTranslations(fc2)

	assert.Equal(t, 2, len(d.LocalToRemoteNamespaceMapper), "the LocalToRemoteNamespaceMapper has to contain an entry for each remote cluster")
	assert.Equal(t, 2, len(d.RemoteToLocalNamespaceMapper), "the RemoteToLocalNamespaceMapper has to contain an entry for each remote cluster")
	assert.Equal(t, 2, len(d.ClusterIDToLocalNamespaceMapper), "the ClusterIDToLocalNamespaceMapper has to contain an entry for each remote cluster")
	assert.Equal(t, 2, len(d.ClusterIDToRemoteNamespaceMapper), "the ClusterIDToRemoteNamespaceMapper has to contain an entry for each remote cluster")

	assert.Equal(t, "remote", d.localToRemoteNamespace("local-1"))
	assert.Equal(t, "remote", d.localToRemoteNamespace("local-2"))

	assert.Equal(t, "local-1", d.remoteToLocalNamespace("cluster-1", "remote"))
	assert.Equal(t, "local-2", d.remoteToLocalNamespace("cluster-2", "remote"))

	assert.Equal(t, "local-1", func() string { v, _ := d.clusterIDToLocalNamespace("cluster-1"); return v }())
	assert.Equal(t, "local-2", func() string { v, _ := d.clusterIDToLocalNamespace("cluster-2"); return v }())

	assert.Equal(t, "remote", func() string { v, _ := d.clusterIDToRemoteNamespace("cluster-1"); return v }())
	assert.Equal(t, "remote", func() string { v, _ := d.clusterIDToRemoteNamespace("cluster-2"); return v }())
}
