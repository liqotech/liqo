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

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	"github.com/liqotech/liqo/pkg/utils"
)

// Provision creates a storage asset and returns a PV object representing it.
func (p *liqoLocalStorageProvisioner) Provision(ctx context.Context,
	options controller.ProvisionOptions) (*v1.PersistentVolume, controller.ProvisioningState, error) {
	if utils.IsVirtualNode(options.SelectedNode) {
		return nil, controller.ProvisioningFinished, &controller.IgnoredError{
			Reason: "the local storage provider is not providing storage for remote nodes"}
	}
	// this process is the local liqo storage provider, provision a local PVC.
	return p.provisionLocalPVC(ctx, options)
}
