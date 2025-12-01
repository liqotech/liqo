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

package firewall

// SetDataType is the type of a set element
// +kubebuilder:validation:Enum="ipv4_addr"
type SetDataType string

// Possible SetDataType values.
const (
	SetDataTypeIPAddr SetDataType = "ipv4_addr"
)

// Set represents a nftables set
// +kubebuilder:object:generate=true
type Set struct {
	// Name is the name of the set.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=200
	// +kubebuilder:validation:Pattern=`^[a-zA-Z][a-zA-Z0-9/\\_.]*$`
	Name string `json:"name"`

	// KeyType is the type of the set keys.
	KeyType SetDataType `json:"keyType"`

	// DataType is the type of the set data.
	// +kubebuilder:validation:Optional
	DataType *SetDataType `json:"dataType,omitempty"`

	// Elements are the elements of the set.
	// +kubebuilder:validation:Optional
	Elements []SetElement `json:"elements,omitempty"`
}

// SetElement represents an element of a nftables set
// +kubebuilder:object:generate=true
type SetElement struct {
	// Key is the key of the set element.
	Key string `json:"key"`

	// Data is the data of the set element.
	// +kubebuilder:validation:Optional
	Data *string `json:"data,omitempty"`
}
