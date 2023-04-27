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

package resourceoffercontroller

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
)

// NewResourceOfferController creates and returns a new reconciler for the ResourceOffers.
func NewResourceOfferController(
	mgr manager.Manager,
	identityReader identitymanager.IdentityReader,
	resyncPeriod time.Duration, disableAutoAccept bool) *ResourceOfferReconciler {
	return &ResourceOfferReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),

		identityReader:    identityReader,
		eventsRecorder:    mgr.GetEventRecorderFor("ResourceOffer"),
		disableAutoAccept: disableAutoAccept,
		resyncPeriod:      resyncPeriod,
	}
}
