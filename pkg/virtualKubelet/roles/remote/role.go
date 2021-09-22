// Copyright 2019-2021 The Liqo Authors
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

// Package remote defines the ClusterRole containing the permissions required by the virtual kubelet in the remote cluster.
package remote

// +kubebuilder:rbac:groups="",resources=configmaps;services;secrets;pods,verbs=get;list;watch;update;patch;delete;create
// +kubebuilder:rbac:groups="",resources=pods/status;services/status,verbs=get;update;patch;list;watch;delete;create
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list;watch

// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;update;create;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete
