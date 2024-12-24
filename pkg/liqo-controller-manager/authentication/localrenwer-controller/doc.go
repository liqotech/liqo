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

// Package localrenwercontroller implements the controller for managing certificate renewals
// for local Identity resources. It monitors Identity objects and creates Renew objects
// when certificates need to be renewed.
//
// The controller is responsible for:
// * Monitoring Identity objects and their certificates
// * Determining when certificates need renewal based on their lifetime
// * Creating and managing Renew objects for certificate renewal
// * Handling manual renewal requests via the "liqo.io/renew" annotation
//
// Certificate renewal is triggered in two ways:
// 1. Automatically when a certificate reaches 2/3 of its lifetime
// 2. Manually when an Identity is annotated with "liqo.io/renew: true"
//
// The controller implements an adaptive requeue mechanism that adjusts the check frequency
// based on how close the certificate is to requiring renewal.
package localrenwercontroller
