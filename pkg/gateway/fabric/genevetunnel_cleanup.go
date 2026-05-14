// Copyright 2019-2026 The Liqo Authors
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

package fabric

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	internalnetwork "github.com/liqotech/liqo/pkg/liqo-controller-manager/networking/internal-network"
	"github.com/liqotech/liqo/pkg/utils/getters"
	"github.com/liqotech/liqo/pkg/utils/network/geneve"
)

var _ manager.Runnable = &RunnableGeneveCleanup{}

// RunnableGeneveCleanup periodically removes orphan geneve interfaces on the gateway.
type RunnableGeneveCleanup struct {
	Client   client.Client
	Interval time.Duration
}

// NewRunnableGeneveCleanup creates a new RunnableGeneveCleanup.
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
				klog.Warningf("geneve cleanup failed: %v", err)
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

	var errs []error
	for _, interfaceItem := range interfaceList {
		name := interfaceItem.Attrs().Name
		if !strings.HasPrefix(name, internalnetwork.InterfaceNamePrefix) {
			continue
		}
		if _, ok := internalnodesMap[name]; !ok {
			klog.Infof("geneve interface %s is not needed anymore", name)
			if err := geneve.EnsureGeneveInterfaceAbsence(name); err != nil {
				errs = append(errs, fmt.Errorf("failed to delete geneve interface %s: %w", name, err))
				continue
			}
			klog.Infof("geneve interface %s deleted", name)
		}
	}

	return errors.Join(errs...)
}
