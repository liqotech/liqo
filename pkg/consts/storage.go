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

package consts

const (
	// StorageProvisionerName is the name of the liqo storage provisioner.
	StorageProvisionerName = "liqo.io/storage"

	// StorageAvailableLabel is the label used to mark if the liqo storage is available on a virtual node.
	StorageAvailableLabel = "storage.liqo.io/available"

	// VirtualPvcNamespaceLabel is the label used to mark the namespace of a virtual PVC.
	VirtualPvcNamespaceLabel = "storage.liqo.io/virtual-pvc-namespace"
	// VirtualPvcNameLabel is the label used to mark the name of a virtual PVC.
	VirtualPvcNameLabel = "storage.liqo.io/virtual-pvc-name"

	// StorageNamespaceLabel is the label used to mark the liqo storage namespace.
	StorageNamespaceLabel = "liqo.io/storage-provisioner"
)
