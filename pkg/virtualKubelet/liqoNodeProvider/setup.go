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
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	liqoconst "github.com/liqotech/liqo/pkg/consts"
)

const (
	linuxos      = "Linux"
	architecture = "amd64"

	labelNodeExcludeBalancersAlpha = "alpha.service-controller.kubernetes.io/exclude-balancer"
	roleLabelKey                   = "kubernetes.io/role"
	roleLabelValue                 = "agent"
)

// InitConfig is the config passed to initialize the LiqoNodeProvider.
type InitConfig struct {
	HomeConfig      *rest.Config
	RemoteConfig    *rest.Config
	HomeClusterID   liqov1beta1.ClusterID
	RemoteClusterID liqov1beta1.ClusterID
	Namespace       string

	NodeName         string
	InternalIP       string
	DaemonPort       uint16
	Version          string
	ExtraLabels      map[string]string
	ExtraAnnotations map[string]string

	PodProviderStopper   chan struct{}
	InformerResyncPeriod time.Duration
	PingDisabled         bool
	CheckNetworkStatus   bool
}

// NewLiqoNodeProvider creates and returns a new LiqoNodeProvider.
func NewLiqoNodeProvider(cfg *InitConfig) *LiqoNodeProvider {
	return &LiqoNodeProvider{
		localClient:           kubernetes.NewForConfigOrDie(cfg.HomeConfig),
		remoteDiscoveryClient: discovery.NewDiscoveryClientForConfigOrDie(cfg.RemoteConfig),
		dynClient:             dynamic.NewForConfigOrDie(cfg.HomeConfig),

		node:              node(cfg),
		terminating:       false,
		lastAppliedLabels: map[string]string{},

		networkModuleEnabled: false,
		networkReady:         false,
		resyncPeriod:         cfg.InformerResyncPeriod,
		pingDisabled:         cfg.PingDisabled,
		checkNetworkStatus:   cfg.CheckNetworkStatus,

		nodeName:         cfg.NodeName,
		nodeIP:           cfg.InternalIP,
		foreignClusterID: cfg.RemoteClusterID,
		tenantNamespace:  cfg.Namespace,
	}
}

func node(cfg *InitConfig) *corev1.Node {
	lbls := map[string]string{
		corev1.LabelHostname: cfg.NodeName,
		roleLabelKey:         roleLabelValue,

		corev1.LabelOSStable:   strings.ToLower(linuxos),
		corev1.LabelArchStable: architecture,

		liqoconst.TypeLabel:       liqoconst.TypeNode,
		liqoconst.RemoteClusterID: string(cfg.RemoteClusterID),

		corev1.LabelNodeExcludeBalancers: strconv.FormatBool(true),
		labelNodeExcludeBalancersAlpha:   strconv.FormatBool(true),
	}

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cfg.NodeName,
			Labels:      labels.Merge(lbls, cfg.ExtraLabels),
			Annotations: cfg.ExtraAnnotations,
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    liqoconst.VirtualNodeTolerationKey,
				Value:  strconv.FormatBool(true),
				Effect: corev1.TaintEffectNoExecute,
			}},
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion:  cfg.Version,
				Architecture:    architecture,
				OperatingSystem: linuxos,
			},
			Addresses:       []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: cfg.InternalIP}},
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: int32(cfg.DaemonPort)}},
			Capacity:        corev1.ResourceList{},
			Allocatable:     corev1.ResourceList{},
			Conditions:      UnknownNodeConditions(cfg),
		},
	}
}
