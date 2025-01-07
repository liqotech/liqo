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

package utils

import (
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// LogInfoLevel -> level associated with informational messages.
	LogInfoLevel = 0
	// LogDebugLevel -> level associated with debug messages.
	LogDebugLevel = 4
)

// FromResult returns a logger level, given the result of a CreateOrUpdate operation.
func FromResult(result controllerutil.OperationResult) klog.Level {
	if result == controllerutil.OperationResultNone {
		return LogDebugLevel
	}
	return LogInfoLevel
}
