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

package fabricipam

import (
	"context"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ipamutils "github.com/liqotech/liqo/pkg/utils/ipam"
)

var (
	fabricIPAM *IPAM
	ready      bool
	mutex      sync.Mutex
)

// Get retrieve and init the IPAM singleton.
func Get(ctx context.Context, cl client.Client) (*IPAM, error) {
	mutex.Lock()
	defer mutex.Unlock()
	if ready {
		return fabricIPAM, nil
	}

	internalCIDR, err := ipamutils.GetInternalCIDR(ctx, cl, corev1.NamespaceAll)
	if err != nil {
		return nil, err
	}

	fabricIPAM, err = newIPAM(internalCIDR)
	if err != nil {
		return nil, err
	}

	if err := Init(ctx, cl, fabricIPAM); err != nil {
		return nil, err
	}

	ready = true

	return fabricIPAM, nil
}
