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

package consts

// OwnershipType indicates the type of ownership over a resource.
type OwnershipType string

const (
	// OwnershipLocal indicates that the resource is owned by the local cluster.
	OwnershipLocal OwnershipType = "Local"
	// OwnershipShared indicates that the ownership over the resource is shared between the two clusters.
	// In particular:
	// - the spec of the resource is owned by the local cluster.
	// - the status by the remote cluster.
	OwnershipShared OwnershipType = "Shared"

	// ReplicationRequestedLabel is the key of a label indicating whether the given resource should be replicated remotely.
	ReplicationRequestedLabel = "liqo.io/replication"
	// ReplicationOriginLabel is the key of a label indicating the origin cluster of a replicated resource.
	ReplicationOriginLabel = "liqo.io/originID"
	// ReplicationDestinationLabel is the key of a label indicating the destination cluster of a replicated resource.
	ReplicationDestinationLabel = "liqo.io/remoteID"
	// ReplicationStatusLabel is the key of a label indicating that this resource has been created by a remote cluster through replication.
	ReplicationStatusLabel = "liqo.io/replicated"

	// LocalPodLabelKey label key added to all the local pods that have been offloaded/replicated to a remote cluster.
	LocalPodLabelKey = "liqo.io/shadowPod"
	// LocalPodLabelValue value of the label added to the local pods that have been offloaded/replicated to a remote cluster.
	LocalPodLabelValue = "true"

	// ManagedByLabelKey is the label key used to indicate that a given resource is managed by another one.
	ManagedByLabelKey = "liqo.io/managed-by"
	// ManagedByShadowPodValue it the label value used to indicate that a given resource is managed by a ShadowPod.
	ManagedByShadowPodValue = "shadowpod"

	// LocalResourceOwnership label key added to a resource when it is owned by a local component.
	// Ex. Local networkconfigs are owned by the component that creates them. If the resource is replicated in
	// a remote cluster this label is removed by the CRDReplicator.
	LocalResourceOwnership = "liqo.io/ownership"
)
