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

// Package controlplane defines the permission to grant to the Liqo control plane
// of a remote cluster in the Tenant Namespace.
package controlplane

// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=resourceslices/status,verbs=get;update;patch;list;watch;delete;create;deletecollection

// +kubebuilder:rbac:groups=authentication.liqo.io,resources=renews,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=renews/status,verbs=get;update;patch;list;watch;delete;create;deletecollection

// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps/status,verbs=get;update;patch;list;watch;delete;create;deletecollection
