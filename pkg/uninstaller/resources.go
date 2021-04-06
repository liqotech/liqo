package uninstaller

import (
	"context"
	clusterconfigV1alpha1 "github.com/liqotech/liqo/apis/config/v1alpha1"
	discoveryV1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"github.com/liqotech/liqo/apis/net/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type toCheckDeleted struct {
	gvr           schema.GroupVersionResource
	labelSelector metav1.LabelSelector
}

type resultType struct {
	Resource toCheckDeleted
	Success  bool
}

var (
	podGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	toCheck = []toCheckDeleted{
		{
			gvr:           v1alpha1.TunnelEndpointGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
		},
		{
			gvr:           v1alpha1.NetworkConfigGroupVersionResource,
			labelSelector: metav1.LabelSelector{},
		},
		{
			gvr: podGVR,
			labelSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "virtual-kubelet"},
			},
		},
		{
			gvr: podGVR,
			labelSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "broadcaster"},
			},
		},
	}
)

func UnjoinClusters(client dynamic.Interface) error {
	r1 := client.Resource(discoveryV1alpha1.ForeignClusterGroupVersionResource)
	t, err := r1.Namespace("").List(context.TODO(), metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	if err != nil {
		return err
	}
	klog.V(8).Info("Getting ForeignClusters list")
	var foreign discoveryV1alpha1.ForeignClusterList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(t.UnstructuredContent(), &foreign); err != nil {
		return err
	}
	klog.V(8).Infof("Unjoin %v ForeignClusters", len(foreign.Items))
	for _, item := range foreign.Items {
		patch := []byte(`{"spec": {"join": false}}`)
		_, err = r1.Namespace(item.Namespace).Patch(context.TODO(), item.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func DisableBroadcasting(client dynamic.Interface) error {
	r1 := client.Resource(clusterconfigV1alpha1.ClusterConfigGroupVersionResource)
	t, err := r1.Namespace("").List(context.TODO(), metav1.ListOptions{TypeMeta: metav1.TypeMeta{}})
	if err != nil {
		return err
	}
	klog.V(8).Infof("Getting clusterConfigs")
	var clusterconfigs clusterconfigV1alpha1.ClusterConfigList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(t.UnstructuredContent(), &clusterconfigs); err != nil {
		return err
	}
	klog.V(8).Infof("Patching ClusterConfigs")
	for _, item := range clusterconfigs.Items {
		patch := []byte(`{"spec": {"advertisementConfig": { "outgoingConfig" : { "enableBroadcaster" : false}}}}`)
		_, err = r1.Namespace(item.Namespace).Patch(context.TODO(), item.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func CheckReconciliation(client dynamic.Interface, toCheck toCheckDeleted) (bool, error) {
	var (
		objectList  *unstructured.UnstructuredList
		err         error
		labelString string
	)

	if labelString, err = generateLabelString(toCheck.labelSelector); err != nil {
		return false, err
	}
	options := metav1.ListOptions{
		LabelSelector: labelString,
	}

	objectList, err = client.Resource(toCheck.gvr).Namespace("").List(context.TODO(), options)

	if apierrors.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if len(objectList.Items) == 0 {
		klog.V(6).Infof("%s not found", toCheck.gvr)
		return true, nil
	}

	return false, nil

}

func WaitForResources(client dynamic.Interface) error {
	klog.Info("Waiting for Liqo Resources to be correctly deleted")
	var wg sync.WaitGroup
	result := make(chan *resultType, len(toCheck))
	wg.Add(len(toCheck))
	for _, resource := range toCheck {
		go WaitForCompletion(client, resource, result, &wg)
	}
	for elem := range result {
		if !elem.Success {
			klog.Errorf("Error while waiting for %s to be deleted", elem.Resource.gvr.GroupResource())
			return nil
		}
		printLabels, _ := generateLabelString(elem.Resource.labelSelector)
		klog.Infof("%s instances with \"%s\" labels correctly deleted", elem.Resource.gvr.GroupResource(), printLabels)
	}
	wg.Wait()
	close(result)
	return nil
}

func WaitForCompletion(client dynamic.Interface, toCheck toCheckDeleted, result chan *resultType, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	var wError error
	value := false
	var res = &resultType{
		Resource: toCheck,
		Success:  false,
	}
	timeout := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-timeout.C:
			close(quit)
		case <-ticker.C:
			value, wError = CheckReconciliation(client, toCheck)
			if value {
				res.Success = true
				close(quit)
			} else if wError != nil {
				close(quit)
			}
			printLabels, _ := generateLabelString(toCheck.labelSelector)
			klog.Infof("Waiting for %s instances with %s labels to be correctly deleted", toCheck.gvr.GroupResource(), printLabels)
		case <-quit:
			ticker.Stop()
			timeout.Stop()
			result <- res
			return
		}
	}
}

func generateLabelString(labelSelector metav1.LabelSelector) (string, error) {
	labelMap, err := metav1.LabelSelectorAsMap(&labelSelector)
	if err != nil {
		return "", err
	}
	return labels.SelectorFromSet(labelMap).String(), nil
}
