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

package conncheck

import "fmt"

// DuplicateError is an error type for ConnChecker.
// It is returned when an already present sender is added.
type DuplicateError struct {
	Err error
}

// NewDuplicateError returns a new DuplicateError.
func NewDuplicateError(clusterID string) *DuplicateError {
	return &DuplicateError{Err: fmt.Errorf("sender %s already added", clusterID)}
}

// Error returns the error message.
func (e DuplicateError) Error() string {
	return e.Err.Error()
}
