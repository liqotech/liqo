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

// Package incoming defines the permission to be enabled when a ResourceRequest has been accepted,
// this ClusterRole has the permissions required to a remote cluster to manage
// an outgoing peering (incoming for the local cluster),
// when the Pods will be offloaded to the local cluster
package incoming

// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=net.liqo.io,resources=networkconfigs/status,verbs=get;update;patch;list;watch;delete;create;deletecollection

// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps,verbs=get;update;patch;list;watch;delete;create;deletecollection
// +kubebuilder:rbac:groups=virtualkubelet.liqo.io,resources=namespacemaps/status,verbs=get;update;patch;list;watch;delete;create;deletecollection
