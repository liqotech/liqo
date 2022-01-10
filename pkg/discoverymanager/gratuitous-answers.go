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

package discovery

import (
	"context"
	"time"

	"k8s.io/klog/v2"
)

func (discovery *Controller) startGratuitousAnswers(ctx context.Context) {
	for {
		select {
		case <-time.After(12 * time.Second):
			discovery.sendAnswer()
		case <-ctx.Done():
			return
		}
	}
}

func (discovery *Controller) sendAnswer() {
	discovery.serverMux.Lock()
	defer discovery.serverMux.Unlock()
	if discovery.mdnsServerAuth != nil {
		klog.V(5).Infof("Sending a gratuitous mDNS answer")
		discovery.mdnsServerAuth.SendMulticast()
	}
}
