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

package info

import (
	"context"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

// CheckerBase is the base interface for the multiple types of checkers.
type CheckerBase interface {
	// Collect the data from the Liqo installation
	Collect(ctx context.Context, options Options)
	// Get the title of the section retrieve by the checker
	GetTitle() string
	// Get the id to be shown of machine readable output
	GetID() string
	// Return the errors occurred during the collection of the data.
	GetCollectionErrors() []error
}

// Checker is the interface to be implemented by all the checkers that
// collect info about the current instance of Liqo.
type Checker interface {
	CheckerBase
	// Return the collected data using a user friendly output
	Format(options Options) string
	// Get the data collected by the checker
	GetData() interface{}
}

// MultiClusterChecker is the interface to be implemented by all the checkers that
// collect info from multiple clusters.
type MultiClusterChecker interface {
	CheckerBase
	// Get the data collected by the checker for the specified clusterID
	GetDataByClusterID(clusterID liqov1beta1.ClusterID) (interface{}, error)
	// Return the collected data for the specified clusterID using a user friendly output
	FormatForClusterID(clusterID liqov1beta1.ClusterID, options Options) string
}

// CheckerCommon contains the common attributes and functions of the checkers.
type CheckerCommon struct {
	collectionErrors []error
}

// AddCollectionError adds an error to the list of errors occurred while
// collecting the info about a Liqo component.
func (c *CheckerCommon) AddCollectionError(err error) {
	c.collectionErrors = append(c.collectionErrors, err)
}

// GetCollectionErrors returns the errors occurred during the collection of the data.
func (c *CheckerCommon) GetCollectionErrors() []error {
	return c.collectionErrors
}
