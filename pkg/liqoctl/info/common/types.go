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
//

package common

// ModuleStatus represents the status of each of the modules.
type ModuleStatus string

const (
	// ModuleHealthy indicates a module that works as expected.
	ModuleHealthy ModuleStatus = "Healthy"
	// ModuleUnhealthy indicates that there are issues with the module.
	ModuleUnhealthy ModuleStatus = "Unhealthy"
	// ModuleDisabled indicates that the modules is not currently used.
	ModuleDisabled ModuleStatus = "Disabled"
)
