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

package leaderelection

import (
	"context"
	"os"

	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// LabelerOnElection is a function that labels the leader pod with the leader label.
func LabelerOnElection(ctx context.Context, mgr manager.Manager, info *PodInfo) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-mgr.Elected():
		}

		if info.DeploymentName == nil {
			return
		}

		maxRetries := 10
		cl := mgr.GetClient()

		if err := retry.OnError(retry.DefaultBackoff, func(_ error) bool {
			if maxRetries == 0 {
				return false
			}
			maxRetries--
			return true
		}, func() error {
			if err := handleLeaderLabelWithClient(ctx, cl, info); err != nil {
				klog.Error(err, "retrying...")
				return err
			}
			return nil
		}); err != nil {
			klog.Error(err)
			os.Exit(1)
		}
	}()
}
