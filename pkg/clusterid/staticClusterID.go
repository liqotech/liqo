// Copyright 2019-2021 The Liqo Authors
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

package clusterid

type staticClusterID struct {
	id string
}

// NewStaticClusterID returns a clusterID interface compliant object that stores a read-only clusterID.
func NewStaticClusterID(clusterID string) ClusterID {
	return &staticClusterID{
		id: clusterID,
	}
}

// SetupClusterID function not implemented.
func (staticCID *staticClusterID) SetupClusterID(namespace string) error {
	panic("not implemented")
}

// GetClusterID returns the clusterID string.
func (staticCID *staticClusterID) GetClusterID() string {
	return staticCID.id
}
