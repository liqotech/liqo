package tunnelEndpointCreator

import (
	"context"
	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

func (tec *TunnelEndpointCreator) StartForeignClusterWatcher() {

	if !tec.IsConfigured {
		klog.Infof("ForeignClusterWatcher is waiting for the operator to be configured")
		tec.WaitConfig.Wait()
		klog.Infof("Operator configured: ForeignClusterWatcher is now starting")
	}
	started := tec.Manager.GetCache().WaitForCacheSync(context.TODO())
	if !started {
		klog.Errorf("unable to sync caches")
		return
	}
	dynFactory := dynamicinformer.NewDynamicSharedInformerFactory(tec.DynClient, ResyncPeriod)
	go tec.Watcher(dynFactory, discoveryv1alpha1.ForeignClusterGroupVersionResource, cache.ResourceEventHandlerFuncs{
		AddFunc:    tec.ForeignClusterHandlerAdd,
		UpdateFunc: tec.ForeignClusterHandlerUpdate,
		DeleteFunc: tec.ForeignClusterHandlerDelete,
	}, tec.ForeignClusterStopWatcher)
}

func (tec *TunnelEndpointCreator) ForeignClusterHandlerAdd(obj interface{}) {
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting interface to unstructured object")
		return
	}
	fc := &discoveryv1alpha1.ForeignCluster{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, fc)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}
	if fc.Status.Incoming.Joined || fc.Status.Outgoing.Joined {
		_ = tec.createNetConfig(fc)
	} else if !fc.Status.Incoming.Joined && !fc.Status.Outgoing.Joined {
		_ = tec.deleteNetConfig(fc)
	}
}

func (tec *TunnelEndpointCreator) ForeignClusterHandlerUpdate(oldObj interface{}, newObj interface{}) {
	tec.ForeignClusterHandlerAdd(newObj)
}

func (tec *TunnelEndpointCreator) ForeignClusterHandlerDelete(obj interface{}) {
	objUnstruct, ok := obj.(*unstructured.Unstructured)
	if !ok {
		klog.Errorf("an error occurred while converting advertisement obj to unstructured object")
		return
	}
	fc := &discoveryv1alpha1.ForeignCluster{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(objUnstruct.Object, fc)
	if err != nil {
		klog.Errorf("an error occurred while converting resource %s of type %s to typed object: %s", objUnstruct.GetName(), objUnstruct.GetKind(), err)
		return
	}
	_ = tec.deleteNetConfig(fc)
}
