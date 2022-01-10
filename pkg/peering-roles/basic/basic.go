// Copyright 2019-2022 The Liqo Authors
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

// Package basic defines the permission to be enabled with the creation
// of the Tenant Namespace,
// this ClusterRole has the basic permissions to give to a remote cluster
package basic

// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=discovery.liqo.io,resources=resourcerequests/status,verbs=get;update;patch;list;watch;delete;create;deletecollection
