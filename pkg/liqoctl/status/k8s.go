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

package status

import (
	"context"
	"fmt"
)

// k8sStatusCollector knows how to interact with k8s cluster.
type k8sStatusCollector struct {
	checkers []Checker
}

// newK8sStatusCollector returns a new k8sStatusCollector.
func newK8sStatusCollector(options *Options) *k8sStatusCollector {
	return &k8sStatusCollector{
		checkers: []Checker{
			newNamespaceChecker(options.LiqoNamespace, options.KubeClient),
			newPodChecker(options.LiqoNamespace, liqoDeployments, liqoDaemonSets, options.KubeClient),
		},
	}
}

// collectStatus collects the status of each Checker that belongs to the collector.
func (k *k8sStatusCollector) collectStatus(ctx context.Context) error {
	for _, checker := range k.checkers {
		if err := checker.Collect(ctx); err != nil {
			return err
		}
		msg, err := checker.Format()
		if err != nil {
			return err
		}
		fmt.Print(msg)
		if !checker.HasSucceeded() {
			break
		}
	}
	return nil
}
