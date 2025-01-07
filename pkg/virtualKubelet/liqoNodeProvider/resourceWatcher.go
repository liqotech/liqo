// Copyright 2019-2025 The Liqo Authors
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

package liqonodeprovider

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
)

// StartProvider starts the provider with its informers on Liqo resources.
// These informers on sharing and network resources will be used to accordingly
// update the virtual node.
func (p *LiqoNodeProvider) StartProvider(ctx context.Context) (ready chan struct{}) {
	namespace := p.tenantNamespace

	nodeInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		p.dynClient, p.resyncPeriod, corev1.NamespaceAll, func(opt *metav1.ListOptions) {
			opt.FieldSelector = "metadata.name=" + p.nodeName
		})
	nodeInformer := nodeInformerFactory.ForResource(corev1.SchemeGroupVersion.WithResource("nodes")).Informer()
	_, err := nodeInformer.AddEventHandler(getEventHandler(p.reconcileNodeFromNode))
	runtime.Must(err)

	virtualNodeInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		p.dynClient, p.resyncPeriod, namespace, func(opt *metav1.ListOptions) {
			opt.FieldSelector = "metadata.name=" + p.nodeName
		})
	virtualNodeInformer := virtualNodeInformerFactory.ForResource(offloadingv1beta1.VirtualNodeGroupVersionResource).Informer()
	_, err = virtualNodeInformer.AddEventHandler(getEventHandler(p.reconcileNodeFromVirtualNode))
	runtime.Must(err)

	var fcInformerFactory dynamicinformer.DynamicSharedInformerFactory
	if p.checkNetworkStatus {
		fcInformerFactory = dynamicinformer.NewFilteredDynamicSharedInformerFactory(p.dynClient, p.resyncPeriod, corev1.NamespaceAll,
			func(opt *metav1.ListOptions) {
				opt.LabelSelector = consts.RemoteClusterID + "=" + string(p.foreignClusterID)
			})
		fcInformer := fcInformerFactory.ForResource(liqov1beta1.ForeignClusterGroupVersionResource).Informer()
		_, err := fcInformer.AddEventHandler(getEventHandler(p.reconcileNodeFromForeignCluster))
		runtime.Must(err)
	}

	ready = make(chan struct{}, 1)
	go func() {
		<-ready
		go virtualNodeInformerFactory.Start(ctx.Done())
		if p.checkNetworkStatus {
			go fcInformerFactory.Start(ctx.Done())
		}
		klog.Info("Liqo informers started")
	}()

	return ready
}

func getEventHandler(handler func(event watch.Event) error) cache.ResourceEventHandler {
	retryFunc := func(event watch.Event) {
		if err := retry.OnError(retry.DefaultBackoff, func(err error) bool {
			klog.Errorf("Retry on error for event %v - %v", event.Type, err)
			return true
		}, func() error {
			return handler(event)
		}); err != nil {
			klog.Errorf("Error for event %v - %v", event.Type, err)
		}
	}

	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event := watch.Event{
				Object: obj.(*unstructured.Unstructured),
				Type:   watch.Added,
			}
			retryFunc(event)
		},
		UpdateFunc: func(_ interface{}, newObj interface{}) {
			event := watch.Event{
				Object: newObj.(*unstructured.Unstructured),
				Type:   watch.Modified,
			}
			retryFunc(event)
		},
		DeleteFunc: func(obj interface{}) {
			event := watch.Event{
				Object: obj.(*unstructured.Unstructured),
				Type:   watch.Deleted,
			}
			retryFunc(event)
		},
	}
}
