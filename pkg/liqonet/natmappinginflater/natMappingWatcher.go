package natmappinginflater

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	liqonetv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/consts"
)

const (
	resyncPeriod       = 30 * time.Second
	natmappigResources = "natmappings"
)

func (inflater *NatMappingInflater) startNatMappingWatcher() {
	dynFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(inflater.dynClient, resyncPeriod, "", setFilteringLabel)
	go inflater.Watcher(dynFactory, liqonetv1alpha1.GroupVersion.WithResource(natmappigResources), cache.ResourceEventHandlerFuncs{
		DeleteFunc: inflater.natMappingDeleteHandler,
	}, inflater.natMappingInformerStop)
}

// Append to the current label selector the NatMapping label.
func setFilteringLabel(options *metav1.ListOptions) {
	if options.LabelSelector == "" {
		newLabelSelector := []string{consts.NatMappingResourceLabelKey, "=", consts.NatMappingResourceLabelValue}
		options.LabelSelector = strings.Join(newLabelSelector, "")
	}
}

// Delete handler for NatMapping resources.
func (inflater *NatMappingInflater) natMappingDeleteHandler(obj interface{}) {
	// Convert the object to unstructured
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	klog.Errorf("NatMapping resource %s has been deleted. Please don't do that anymore.",
		objUnstruct.GetName())

	retryError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Remove resource version (Create returns error otherwise)
		objUnstruct.SetResourceVersion("")
		// Create the object again
		_, err := inflater.dynClient.Resource(liqonetv1alpha1.NatMappingGroupResource).Create(context.Background(),
			objUnstruct, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		klog.Infof("Resource %s previously deleted, has been created successfully again", objUnstruct.GetName())
		return nil
	})
	if retryError != nil {
		fileName, err := dumpNatMappingOnFile(objUnstruct)
		if err != nil {
			klog.Errorf(`an error occurred while creating previously deleted resource %s: %s`,
				objUnstruct.GetName(), retryError.Error())
			return
		}
		klog.Errorf("an error occurred while creating previously deleted resource %s."+
			"Resource has been be dumped in file %s: %s", objUnstruct.GetName(), fileName, retryError.Error())
	}
}

// Function that dumps yaml version of NatMapping resource on a file.
func dumpNatMappingOnFile(res *unstructured.Unstructured) (string, error) {
	fileName := fmt.Sprintf("%s.yaml", res.GetName())
	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	dump, err := yaml.Marshal(res)
	if err != nil {
		return "", err
	}
	_, err = file.Write(dump)
	if err != nil {
		return "", err
	}
	return fileName, nil
}

// Watcher for resources.
func (inflater *NatMappingInflater) Watcher(sharedDynFactory dynamicinformer.DynamicSharedInformerFactory,
	resourceType schema.GroupVersionResource,
	handlerFuncs cache.ResourceEventHandlerFuncs,
	stopCh chan struct{}) {
	dynInformer := sharedDynFactory.ForResource(resourceType)
	klog.Infof("Starting watcher for %s", resourceType.String())
	// Adding handlers to the informer
	dynInformer.Informer().AddEventHandler(handlerFuncs)
	// Run the informer
	dynInformer.Informer().Run(stopCh)
}
