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

// Package liqons contains the permissions required on the "liqo" namespace to create peering connection with the provider cluster via liqoctl.
package liqons

// +kubebuilder:rbac:groups=core,resources=configmaps,namespace="do-not-care",verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.liqo.io,resources=networks,namespace="do-not-care",verbs=get;list
// +kubebuilder:rbac:groups=networking.liqo.io,resources=wggatewayservertemplates;wggatewayclienttemplates,namespace="do-not-care",verbs=get;list
