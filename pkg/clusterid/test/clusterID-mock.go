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

package test

// ClusterIDMock implements a mock for the ClusterID type.
type ClusterIDMock struct {
	Id string
}

// SetupClusterID sets a new clusterid.
func (cId *ClusterIDMock) SetupClusterID(namespace string) error {
	cId.Id = "local-cluster"
	return nil
}

// GetClusterID retrieves the clusterid.
func (cId *ClusterIDMock) GetClusterID() string {
	return cId.Id
}
