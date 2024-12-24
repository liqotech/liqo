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

// Package remoterenwercontroller implements the controller for handling certificate renewal requests
// from remote clusters. It processes Renew objects created by remote clusters and generates
// new certificates based on the provided CSR.
//
// The controller is responsible for:
// * Processing Renew objects created by remote clusters
// * Validating the renewal request against the tenant namespace
// * Generating new certificates using the provided CSR
// * Updating the status of related resources (Tenant or ResourceSlice)
// * Managing the lifecycle of Renew objects
//
// The renewal process is triggered either by certificate expiration (2/3 lifetime rule)
// or manually through the "liqo.io/renew" annotation.
package remoterenwercontroller
