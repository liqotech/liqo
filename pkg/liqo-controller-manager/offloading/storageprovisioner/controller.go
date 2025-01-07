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

package storageprovisioner

import (
	"context"

	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"
)

// StorageControllerRunnable wraps the storage ProvisionController to implement the Runnable interface.
type StorageControllerRunnable struct {
	Ctrl *controller.ProvisionController
}

// Start starts the runnable and make it run until the context is open.
func (c StorageControllerRunnable) Start(ctx context.Context) error {
	// The Run method blocks forever, regardless of the context status.
	// Hence, this is executed in a goroutine, to ensure the method terminates when the context is closed.
	go c.Ctrl.Run(ctx)
	<-ctx.Done()
	return nil
}
