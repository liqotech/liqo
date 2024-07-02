// Copyright 2019-2024 The Liqo Authors
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
	liqov1alpha1 "github.com/liqotech/liqo/apis/core/v1alpha1"
)

// SetRole sets the role of a foreign cluster.
func SetRole(foreignCluster *liqov1alpha1.ForeignCluster, consumer, provider bool) {
	switch {
	case consumer && provider:
		foreignCluster.Status.Role = liqov1alpha1.ConsumerAndProviderRole
	case consumer:
		foreignCluster.Status.Role = liqov1alpha1.ConsumerRole
	case provider:
		foreignCluster.Status.Role = liqov1alpha1.ProviderRole
	default:
		foreignCluster.Status.Role = liqov1alpha1.UnknownRole
	}
}

// IsProvider checks if a foreign cluster is a provider.
func IsProvider(foreignCluster *liqov1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Role == liqov1alpha1.ProviderRole || foreignCluster.Status.Role == liqov1alpha1.ConsumerAndProviderRole
}

// IsConsumer checks if a foreign cluster is a consumer.
func IsConsumer(foreignCluster *liqov1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Role == liqov1alpha1.ConsumerRole || foreignCluster.Status.Role == liqov1alpha1.ConsumerAndProviderRole
}

// IsConsumerAndProvider checks if a foreign cluster is both a consumer and a provider.
func IsConsumerAndProvider(foreignCluster *liqov1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Role == liqov1alpha1.ConsumerAndProviderRole
}

// IsUnknown checks if a foreign cluster has an unknown role.
func IsUnknown(foreignCluster *liqov1alpha1.ForeignCluster) bool {
	return foreignCluster.Status.Role == liqov1alpha1.UnknownRole
}
