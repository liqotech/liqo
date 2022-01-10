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

package discovery

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	discoveryPkg "github.com/liqotech/liqo/pkg/discovery"
	foreignclusterutils "github.com/liqotech/liqo/pkg/utils/foreignCluster"
)

func (discovery *Controller) startGarbageCollector(ctx context.Context) {
	for {
		select {
		case <-time.After(30 * time.Second):
			_ = discovery.collectGarbage(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// The GarbageCollector deletes all ForeignClusters discovered with LAN and WAN that have expired TTL.
func (discovery *Controller) collectGarbage(ctx context.Context) error {
	req, err := labels.NewRequirement(discoveryPkg.DiscoveryTypeLabel, selection.In, []string{
		string(discoveryPkg.LanDiscovery),
		string(discoveryPkg.WanDiscovery),
	})
	utilruntime.Must(err)

	var fcs discoveryv1alpha1.ForeignClusterList
	if err := discovery.List(ctx, &fcs, &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*req),
	}); err != nil {
		klog.Error(err)
		return err
	}

	for i := range fcs.Items {
		if foreignclusterutils.IsExpired(&fcs.Items[i]) {
			klog.V(4).Infof("delete foreignCluster %v (TTL expired)", fcs.Items[i].Name)
			klog.Infof("delete foreignCluster %v", fcs.Items[i].Name)
			if err := discovery.Delete(ctx, &fcs.Items[i]); err != nil {
				klog.Error(err)
				continue
			}
		}
	}
	return nil
}
