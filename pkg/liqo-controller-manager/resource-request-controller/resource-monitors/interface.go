// Copyright 2019-2023 The Liqo Authors
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
)

const (
	// AllClusterIDs useful to update all the clusters.
	AllClusterIDs = ""
)

// ResourceUpdateNotifier represents an interface for OfferUpdater to receive resource updates.
type ResourceUpdateNotifier interface {
	// NotifyChange signals that a change in resources may have occurred.
	NotifyChange(clusterID string)
}

// ResourceReader represents an interface to read the available resources in this cluster.
type ResourceReader interface {
	// ReadResources returns the resources available for usage by the given cluster.
	ReadResources(ctx context.Context, clusterID string) ([]*ResourceList, error)
	// Register sets the component that will be notified of changes.
	Register(context.Context, ResourceUpdateNotifier)
	// RemoveClusterID removes the given clusterID from all internal structures.
	RemoveClusterID(ctx context.Context, clusterID string) error
}
