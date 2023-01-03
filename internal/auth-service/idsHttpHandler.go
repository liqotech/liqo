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

package authservice

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"k8s.io/klog/v2"
	"k8s.io/utils/trace"

	"github.com/liqotech/liqo/pkg/auth"
)

// this HTTP handler returns home cluster information to the foreign clusters that are asking for them,
// it returns a JSON encoded ClusterInfo struct with the following fields:
// - clusterID		-> the id of the home cluster.
// - clusterName	-> the custom name for the home cluster (to be displayed in GUIs).
func (authService *Controller) ids(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	tracer := trace.New("IDs handler")
	defer tracer.LogIfLong(10 * time.Millisecond)

	idsResponse := authService.getIdsResponse()

	res, err := json.Marshal(idsResponse)
	if err != nil {
		klog.Error(err)
		authService.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write(res); err != nil {
		klog.Error(err)
		return
	}
}

func (authService *Controller) getIdsResponse() *auth.ClusterInfo {
	return &auth.ClusterInfo{
		ClusterID:   authService.localCluster.ClusterID,
		ClusterName: authService.localCluster.ClusterName,
	}
}
