// Copyright 2019-2023 The Liqo Authors
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

package ipam

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

var nodeIpam *IPAM
var nodeIpamOnce sync.Once

var nodeNetwork string

// GetNodeIpam returns the IPAM for the internal nodes or creates it if not exists. It is a singleton.
func GetNodeIpam(ctx context.Context, cl client.Client) (*IPAM, error) {
	if nodeNetwork == "" {
		// TODO: get from network CRD
		nodeNetwork = "10.201.0.0/16"
	}

	nodeIpamOnce.Do(func() {
		var err error
		nodeIpam, err = New(nodeNetwork)
		runtime.Must(err)

		var internalNodes networkingv1alpha1.InternalNodeList
		err = cl.List(ctx, &internalNodes)
		runtime.Must(err)

		for i := range internalNodes.Items {
			intNode := &internalNodes.Items[i]
			err = nodeIpam.Configure(intNode.Name, intNode.Spec.IP.String())
			runtime.Must(err)
		}
	})

	return nodeIpam, nil
}
