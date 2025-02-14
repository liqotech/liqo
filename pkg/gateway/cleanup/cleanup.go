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

package cleanup

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/network/geneve"
)

var _ manager.Runnable = &RunnableGeneveCleanup{}

// RunnableGeneveCleanup is a RunnableGeneveCleanup that manages concurrency.
type RunnableGeneveCleanup struct {
	Client   client.Client
	Interval time.Duration
}

// NewRunnableGeneveCleanup creates a new Runnable.
func NewRunnableGeneveCleanup(cl client.Client, interval time.Duration) (*RunnableGeneveCleanup, error) {
	return &RunnableGeneveCleanup{
		Client:   cl,
		Interval: interval,
	}, nil
}

// Start implements manager.Runnable.
func (rgc *RunnableGeneveCleanup) Start(ctx context.Context) error {
	klog.Infof("Running geneve cleanup every %s", rgc.Interval)

	ticker := time.NewTicker(rgc.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := geneveCleanup(ctx, rgc.Client); err != nil {
				return fmt.Errorf("geneve cleanup failed: %w", err)
			}
		}
	}
}

func geneveCleanup(ctx context.Context, cl client.Client) error {
	interfaceList, err := geneve.ListGeneveInterfaces()
	if err != nil {
		return fmt.Errorf("failed to list geneve interfaces: %w", err)
	}

	internalnodesList, err := getters.ListInternalNodesByLabels(ctx, cl, labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list internal nodes: %w", err)
	}

	internalnodesMap := make(map[string]any)
	for i := range internalnodesList.Items {
		internalnodesMap[internalnodesList.Items[i].Spec.Interface.Gateway.Name] = struct{}{}
	}

	for _, interfaceItem := range interfaceList {
		if _, ok := internalnodesMap[interfaceItem.Attrs().Name]; !ok {
			klog.Infof("geneve interface %s is not needed anymore", interfaceItem.Attrs().Name)
			if err := geneve.EnsureGeneveInterfaceAbsence(interfaceItem.Attrs().Name); err != nil {
				return fmt.Errorf("failed to delete geneve interface %s: %w", interfaceItem, err)
			}
			klog.Infof("geneve interface %s deleted", interfaceItem.Attrs().Name)
		}
	}

	return nil
}
