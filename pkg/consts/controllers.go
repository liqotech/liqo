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

package consts

// Constants used to name and identify controllers.
// Controller-runtime requires that each controller has a unique name within the container.
// This name is used, for example, to identify a controller univocally in the logs
// and must be a prometheus compatible name (underscores and alphanumeric characters only).
// As a convention to avoid conflicts, we use the name of the reconciled resource (lowercase version of their kind),
// and, if already used, we add a recognizable identifier, separated by the underscore character.
// To catch duplicated names, we name the constant as its value (in CamelCase and stripping the separator character),
// with the prefix "Ctrl".
const (
	// Core.
	CtrlSecretsCRDReplicator = "secrets_crdreplicator"

	// Networking.
	CtrlInternalFabricsFabric = "internalfabrics_fabric"
	CtrlInternalFabricsCM     = "internalfabrics_cm"

	// Authentication.
	CtrlResourceSlicesLocal  = "resourceslices_local"
	CtrlResourceSlicesRemote = "resourceslices_remote"

	// Offloading.
	CtrlShadowPods = "shadowpods"

	// Cross modules.
	CtrlResourceSlicesQuotaCreator = "resourceslices_quotacreator"
)
