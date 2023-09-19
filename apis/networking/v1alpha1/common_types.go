// Copyright 2019-2023 The Liqo Authors
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

// Package v1alpha1 contains API Schema definitions for the networking v1alpha1 API group.
//
//nolint:lll // ignore long lines given by Kubebuilder marker annotations.
package v1alpha1

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CIDR defines a syntax validated CIDR.
// +kubebuilder:validation:Pattern=`^(([1-9]{0,1}[0-9]{0,2}|2[0-4][0-9]|25[0-5])\.){3}([1-9]{0,1}[0-9]{0,2}|2[0-4][0-9]|25[0-5])\/([0-9]|[1-2][0-9]|3[0-2])$`
type CIDR string

func (c CIDR) String() string {
	return string(c)
}

// IP defines a syntax validated IP.
// +kubebuilder:validation:Pattern=`^(([1-9]{0,1}[0-9]{0,2}|2[0-4][0-9]|25[0-5])\.){3}([1-9]{0,1}[0-9]{0,2}|2[0-4][0-9]|25[0-5])$`
type IP string

func (i IP) String() string {
	return string(i)
}
