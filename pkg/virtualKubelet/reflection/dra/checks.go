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

package dra

import (
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

// draAPIGroupVersion is the API group/version for Kubernetes Dynamic Resource
// Allocation. Reflectors are only registered when both clusters expose this group.
const draAPIGroupVersion = "resource.k8s.io/v1"

// IsDRASupportedOnBothClusters returns true only when resource.k8s.io/v1 (with the
// resourceslices, resourceclaims and deviceclasses resources) is available on both the
// local and the remote cluster.
func IsDRASupportedOnBothClusters(local, remote kubernetes.Interface) (bool, error) {
	okLocal, err := isDRAAPISupported(local)
	if err != nil {
		return false, err
	}
	if !okLocal {
		return false, nil
	}
	return isDRAAPISupported(remote)
}

func isDRAAPISupported(client kubernetes.Interface) (bool, error) {
	res, err := client.Discovery().ServerResourcesForGroupVersion(draAPIGroupVersion)
	if err != nil {
		if discovery.IsGroupDiscoveryFailedError(err) || kerrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	present := map[string]bool{
		"resourceslices": false,
		"resourceclaims": false,
		"deviceclasses":  false,
	}
	for i := range res.APIResources {
		present[res.APIResources[i].Name] = true
	}
	for _, ok := range present {
		if !ok {
			return false, nil
		}
	}
	return true, nil
}
