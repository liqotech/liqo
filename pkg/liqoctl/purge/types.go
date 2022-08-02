// Copyright 2019-2022 The Liqo Authors
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

package purge

import (
	"time"

	"github.com/pterm/pterm"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
)

// Args contains the arguments for the purge command.
type Args struct {
	Config1       string
	Config2       string
	RemoteCluster string
	Timeout       time.Duration
}

type clusterHandler struct {
	number int
	color  pterm.Color
	//printer               *common.Printer
	cl                    client.Client
	nativeCl              kubernetes.Interface
	localClusterIdentity  discoveryv1alpha1.ClusterIdentity
	remoteClusterIdentity discoveryv1alpha1.ClusterIdentity
	hasFullClusterAccess  bool
}

type handler struct {
	handler1 *clusterHandler
	handler2 *clusterHandler
}
