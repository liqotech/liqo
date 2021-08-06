package authservice

import (
	"encoding/json"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/auth"
)

// this HTTP handler returns home cluster information to the foreign clusters that are asking for them,
// it returns a JSON encoded ClusterInfo struct with the following fields:
// - clusterID		-> the id of the home cluster.
// - clusterName	-> the custom name for the home cluster (to be displayed in GUIs).
// - guestNamespace	-> the namespace where to create secrets and resources to be shared with the home cluster.
func (authService *Controller) ids(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
	conf := authService.getDiscoveryConfig()
	return &auth.ClusterInfo{
		ClusterID:   authService.localClusterID.GetClusterID(),
		ClusterName: conf.ClusterName,
	}
}
