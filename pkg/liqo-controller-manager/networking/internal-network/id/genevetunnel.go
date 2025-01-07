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

package id

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

var geneveTunnelManager *Manager[uint32]
var geneveTunnelOnce sync.Once

// GetGeneveTunnelManager returns the Manager for Geneve tunnels' ids or creates it if not exists. It is a singleton.
func GetGeneveTunnelManager(ctx context.Context, cl client.Client) *Manager[uint32] {
	geneveTunnelOnce.Do(func() {
		geneveTunnelManager = New[uint32]()

		var tunnelList networkingv1beta1.GeneveTunnelList
		err := cl.List(ctx, &tunnelList)
		runtime.Must(err)

		for i := range tunnelList.Items {
			tunnel := &tunnelList.Items[i]
			err = geneveTunnelManager.Configure(tunnel.Name, tunnel.Spec.ID)
			runtime.Must(err)
		}
	})
	return geneveTunnelManager
}
