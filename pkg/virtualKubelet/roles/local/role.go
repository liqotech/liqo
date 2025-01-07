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

// Package local defines the ClusterRole containing the permissions required by the virtual kubelet in the local cluster.
package local

// +kubebuilder:rbac:groups=core,resources=configmaps;services;services/status;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes;nodes/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;list;watch;create;delete;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/eviction,verbs=create
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims;persistentvolumes,verbs=get;list;watch;create;delete;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts/token,verbs=create
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch

// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch

// +kubebuilder:rbac:groups=offloading.liqo.io,resources=namespacemaps;virtualnodes,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.liqo.io,resources=foreignclusters/status,verbs=get;list;watch

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;delete

// Additional permissions necessary for the virtual kubelet initialization process.
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;update;patch
// +kubebuilder:rbac:groups=certificates.k8s.io,resources=certificatesigningrequests,verbs=create;get;list;watch

// Additional permissions necessary for the networking module
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=get;list;watch
// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations,verbs=get;list;watch
