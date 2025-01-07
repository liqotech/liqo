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

package remotemetrics

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/julienschmidt/httprouter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	basePath = "/apis/metrics.liqo.io/v1beta1"
)

var (
	availablePaths = []string{
		"metrics",
		"metrics/cadvisor",
		"metrics/resource",
		"metrics/probes",
	}
)

type metricHandler struct {
	*httprouter.Router
	scraper Scraper
}

// GetHTTPHandler returns a handler for the metrics API.
func GetHTTPHandler(restClient rest.Interface, cl client.Client) (http.Handler, error) {
	router := &metricHandler{
		Router:  httprouter.New(),
		scraper: NewAPIServiceScraper(restClient, cl),
	}

	// Return empty api resource list.
	// K8s expects to be able to retrieve a resource list for each aggregated
	// app in order to discover what resources it provides.
	router.GET("/", health)
	// K8s needs the ability to query info about a specific API group
	router.GET("/apis/metrics.liqo.io", apiGroupInfo)
	// K8s needs the ability to query the list of API groups this endpoint supports
	router.GET("/apis", apiGroupList)

	router.GET(basePath, health)
	router.GET(fmt.Sprintf("%s/scrape/:cluster-id/:path", basePath), router.metricHTTP)
	router.GET(fmt.Sprintf("%s/scrape/:cluster-id/:path/:subpath", basePath), router.metricHTTP)

	return router, nil
}

func health(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	list := &metav1.APIResourceList{}

	list.Kind = "APIResourceList"
	list.GroupVersion = "metrics.liqo.io/v1beta1"
	list.APIVersion = "v1beta1"
	list.APIResources = []metav1.APIResource{
		{
			Name:       "scrape/metrics",
			Namespaced: false,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(list); err != nil {
		klog.Errorf("failed to write response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func getAPIGroup() *metav1.APIGroup {
	return &metav1.APIGroup{
		TypeMeta: metav1.TypeMeta{
			Kind: "APIGroup",
		},
		Name: "metrics.liqo.io",
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: "metrics.liqo.io/v1beta1",
			Version:      "v1beta1",
		},
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: "metrics.liqo.io/v1beta1",
				Version:      "v1beta1",
			},
		},
		ServerAddressByClientCIDRs: []metav1.ServerAddressByClientCIDR{
			{
				ClientCIDR:    "0.0.0.0/0",
				ServerAddress: "",
			},
		},
	}
}

func apiGroupInfo(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(getAPIGroup()); err != nil {
		klog.Errorf("failed to write response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func apiGroupList(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	list := &metav1.APIGroupList{}
	list.Kind = "APIGroupList"
	list.Groups = append(list.Groups, *getAPIGroup())
	if err := json.NewEncoder(w).Encode(list); err != nil {
		klog.Errorf("failed to write response: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (handler *metricHandler) metricHTTP(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	ctx := req.Context()

	clusterID := ps.ByName("cluster-id")
	path := ps.ByName("path")
	subpath := ps.ByName("subpath")
	if subpath != "" {
		path = path + "/" + subpath
	}

	if !handler.isValidPath(path) {
		klog.Errorf("invalid path: %s", path)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	metrics, err := handler.scraper.Scrape(ctx, path, clusterID)
	if err != nil {
		klog.Errorf("failed to scrape metrics: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err = w.Write([]byte(err.Error())); err != nil {
			klog.Errorf("failed to write error: %s", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	metrics.Write(w)
}

func (handler *metricHandler) isValidPath(path string) bool {
	for _, p := range availablePaths {
		if path == p {
			return true
		}
	}
	return false
}
