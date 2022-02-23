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

package resourcemonitors

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceScaler scales the resources of a ResourceReader by a given amount.
// It is used to let one reserve resources for local usage and not share them (Factor < 1).
type ResourceScaler struct {
	Provider ResourceReader
	Factor   float32
	Notifier ResourceUpdateNotifier
}

// Register sets an update notifier.
func (s *ResourceScaler) Register(ctx context.Context, notifier ResourceUpdateNotifier) {
	s.Notifier = notifier
	s.Provider.Register(ctx, notifier)
}

// ReadResources returns the provider's resources scaled by the given amount.
func (s *ResourceScaler) ReadResources(clusterID string) corev1.ResourceList {
	resources := s.Provider.ReadResources(clusterID)
	for resourceName, quantity := range resources {
		scaled := quantity
		ScaleResources(resourceName, &scaled, s.Factor)
		resources[resourceName] = scaled
	}
	return resources
}

// RemoveClusterID removes the given clusterID from the provider.
func (s *ResourceScaler) RemoveClusterID(clusterID string) {
	s.Provider.RemoveClusterID(clusterID)
}

// ScaleResources multiplies a resource by a factor.
func ScaleResources(resourceName corev1.ResourceName, quantity *resource.Quantity, factor float32) {
	switch resourceName {
	case corev1.ResourceCPU:
		// use millis
		quantity.SetScaled(int64(float32(quantity.MilliValue())*factor), resource.Milli)
	case corev1.ResourceMemory:
		// use mega
		quantity.SetScaled(int64(float32(quantity.ScaledValue(resource.Mega))*factor), resource.Mega)
	default:
		quantity.Set(int64(float32(quantity.Value()) * factor))
	}
}
