// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tunnelEndpointCreator

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	foreigncluster "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

func (tec *TunnelEndpointCreator) StartForeignClusterWatcher() {
	if !tec.IsConfigured {
		klog.Infof("ForeignClusterWatcher is waiting for the operator to be configured")
		tec.WaitConfig.Wait()
		klog.Infof("Operator configured: ForeignClusterWatcher is now starting")
	}
	ctx := context.Background()
	started := tec.Manager.GetCache().WaitForCacheSync(ctx)
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
	if foreigncluster.IsIncomingJoined(fc) || foreigncluster.IsOutgoingJoined(fc) {
		_ = tec.createNetConfig(fc)
	} else if !foreigncluster.IsIncomingJoined(fc) && !foreigncluster.IsOutgoingJoined(fc) {
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
