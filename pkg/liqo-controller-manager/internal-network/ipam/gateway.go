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
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
)

var gatewatIpam *IPAM
var gatewatIpamOnce sync.Once

var gatewayNetwork string

// GetGatewayIpam returns the IPAM for the gateway or creates it if not exists. It is a singleton.
func GetGatewayIpam(ctx context.Context, cl client.Client) (*IPAM, error) {
	if gatewayNetwork == "" {
		// TODO: get from network CRD
		gatewayNetwork = "10.200.0.0/16"
	}

	gatewatIpamOnce.Do(func() {
		var err error
		gatewatIpam, err = New(gatewayNetwork)
		runtime.Must(err)

		var internalFabrics networkingv1alpha1.InternalFabricList
		err = cl.List(ctx, &internalFabrics)
		runtime.Must(err)

		for i := range internalFabrics.Items {
			intFab := &internalFabrics.Items[i]
			if intFab.Spec.GatewayIP != "" {
				err = gatewatIpam.Configure(fmt.Sprintf("%s/%s", intFab.Namespace, intFab.Name), string(intFab.Spec.GatewayIP))
				runtime.Must(err)
			}
		}
	})

	return gatewatIpam, nil
}
