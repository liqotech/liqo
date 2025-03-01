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

// Package tenantns contains the permissions required on the tenant namespace to create peering connection with the provider cluster via liqoctl.
package tenantns

// +kubebuilder:rbac:groups=networking.liqo.io,resources=configurations;gatewayclients;gatewayservers;publickeies,verbs=create;update;get;list;delete
// +kubebuilder:rbac:groups=networking.liqo.io,resources=connections,verbs=get;list
// +kubebuilder:rbac:groups=networking.liqo.io,resources=gatewayclients/status;gatewayservers/status,verbs=get
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=ips,verbs=create;update;get;delete
// +kubebuilder:rbac:groups=authentication.liqo.io,resources=tenants,verbs=create;update;delete;get;list;
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=create;get;list;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get
