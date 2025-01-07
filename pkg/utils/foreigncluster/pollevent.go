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

package foreigncluster

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

type fcEventChecker func(fc *liqov1beta1.ForeignCluster) bool

// PollForEvent polls until the given events occurs on the foreign cluster corresponding to the identity.
func PollForEvent(ctx context.Context, cl client.Client, id liqov1beta1.ClusterID,
	checker fcEventChecker, interval time.Duration) error {
	err := wait.PollImmediateUntilWithContext(ctx, interval, func(ctx context.Context) (done bool, err error) {
		fc, err := GetForeignClusterByID(ctx, cl, id)
		if err != nil {
			return false, err
		}

		return checker(fc), nil
	})

	if err != nil {
		return err
	}
	return nil
}
