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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *liqoLocalStorageProvisioner) Delete(ctx context.Context, volume *v1.PersistentVolume) error {
	realPvc := v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      string(volume.Spec.ClaimRef.UID),
			Namespace: p.storageNamespace,
		},
	}

	return client.IgnoreNotFound(p.client.Delete(ctx, &realPvc))
}
