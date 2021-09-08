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

package forge

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1beta1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
	liqonetIpam "github.com/liqotech/liqo/pkg/liqonet/ipam"
	"github.com/liqotech/liqo/pkg/virtualKubelet/namespacesmapping"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options"
	"github.com/liqotech/liqo/pkg/virtualKubelet/options/types"
)

func ForeignToHomeStatus(foreignObj, homeObj runtime.Object) (runtime.Object, error) {
	switch foreignObj.(type) {
	case *corev1.Pod:
		return forger.podStatusForeignToHome(foreignObj, homeObj), nil
	}

	return nil, errors.Errorf("error while creating home object status from foreign: api %s unhandled", reflect.TypeOf(foreignObj).String())
}

func ForeignToHome(foreignObj, homeObj runtime.Object, reflectionType string) (runtime.Object, error) {
	switch foreignObj.(type) {
	case *corev1.Pod:
		return forger.podForeignToHome(foreignObj, homeObj, reflectionType)
	}

	return nil, errors.Errorf("error while creating home object from foreign: api %s unhandled", reflect.TypeOf(foreignObj).String())
}

func HomeToForeign(homeObj, foreignObj runtime.Object, reflectionType string) (runtime.Object, error) {
	switch homeObj.(type) {
	case *corev1.ConfigMap:
		return forger.configmapHomeToForeign(homeObj.(*corev1.ConfigMap), foreignObj.(*corev1.ConfigMap))
	case *discoveryv1beta1.EndpointSlice:
		return forger.endpointsliceHomeToForeign(homeObj.(*discoveryv1beta1.EndpointSlice), foreignObj.(*discoveryv1beta1.EndpointSlice))
	case *corev1.Pod:
		return forger.podHomeToForeign(homeObj, foreignObj, reflectionType)
	case *corev1.Service:
		return forger.serviceHomeToForeign(homeObj.(*corev1.Service), foreignObj.(*corev1.Service))
	}

	return nil, errors.Errorf("error while creating foreign object from home: api %s unhandled", reflect.TypeOf(homeObj).String())
}

func ReplicasetFromPod(pod *corev1.Pod) *appsv1.ReplicaSet {
	return forger.replicasetFromPod(pod)
}

func ForeignReplicasetDeleted(pod *corev1.Pod) *corev1.Pod {
	return forger.setPodToBeDeleted(pod)
}

type apiForger struct {
	nattingTable namespacesmapping.NamespaceNatter
	ipamClient   liqonetIpam.IpamClient

	virtualNodeName  options.ReadOnlyOption
	liqoIpamServer   options.ReadOnlyOption
	offloadClusterID options.ReadOnlyOption
}

var forger apiForger

// InitForger initialize forger component to set all necessary fields of offloaded resources.
func InitForger(nattingTable namespacesmapping.NamespaceNatter, opts ...options.ReadOnlyOption) {
	forger.nattingTable = nattingTable
	for _, opt := range opts {
		switch opt.Key() {
		case types.VirtualNodeName:
			forger.virtualNodeName = opt
		case types.RemoteClusterID:
			forger.offloadClusterID = opt
		case types.LiqoIpamServer:
			forger.liqoIpamServer = opt
			initIpamClient()
		}
	}
}

func initIpamClient() {
	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", forger.liqoIpamServer.Value().ToString(), consts.NetworkManagerIpamPort),
		grpc.WithInsecure(),
		grpc.WithBlock())
	if err != nil {
		klog.Error(err)
	}
	forger.ipamClient = liqonetIpam.NewIpamClient(conn)
}
