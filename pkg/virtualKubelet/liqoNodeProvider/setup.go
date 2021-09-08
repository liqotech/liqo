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

package liqonodeprovider

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

// NewLiqoNodeProvider creates and returns a new LiqoNodeProvider.
func NewLiqoNodeProvider(
	nodeName, foreignClusterID, kubeletNamespace string,
	node *v1.Node,
	podProviderStopper, networkReadyChan chan struct{},
	config *rest.Config, resyncPeriod time.Duration) (*LiqoNodeProvider, error) {
	if config == nil {
		config = ctrl.GetConfigOrDie()
	}
	client := kubernetes.NewForConfigOrDie(config)
	dynClient := dynamic.NewForConfigOrDie(config)

	return &LiqoNodeProvider{
		client:    client,
		dynClient: dynClient,

		node:              node,
		terminating:       false,
		lastAppliedLabels: map[string]string{},

		networkReady:       false,
		podProviderStopper: podProviderStopper,
		networkReadyChan:   networkReadyChan,
		resyncPeriod:       resyncPeriod,

		nodeName:         nodeName,
		foreignClusterID: foreignClusterID,
		kubeletNamespace: kubeletNamespace,
	}, nil
}
