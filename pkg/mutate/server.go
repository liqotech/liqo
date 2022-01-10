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

package mutate

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	cachedclient "github.com/liqotech/liqo/pkg/utils/cachedClient"
)

type MutationConfig struct {
	CertFile string
	KeyFile  string
}

type MutationServer struct {
	mux    *http.ServeMux
	server *http.Server

	webhookClient client.Client
	config        *MutationConfig
	ctx           context.Context
}

// NewMutationServer creates a new mutation server.
func NewMutationServer(ctx context.Context, c *MutationConfig) (*MutationServer, error) {
	s := &MutationServer{}
	s.config = c
	s.ctx = ctx

	// This scheme is necessary for the WebhookClient.
	scheme := runtime.NewScheme()
	_ = offv1alpha1.AddToScheme(scheme)

	var err error
	if s.webhookClient, err = cachedclient.GetCachedClient(ctx, scheme); err != nil {
		return nil, err
	}

	s.mux = http.NewServeMux()
	s.mux.HandleFunc("/mutate", s.handleMutate)

	s.server = &http.Server{
		Addr:           ":8443",
		Handler:        s.mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1048576
	}

	return s, nil
}

func (s *MutationServer) handleMutate(w http.ResponseWriter, r *http.Request) {
	// read the body / request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		klog.Error(err)
		standardErrMessage := fmt.Errorf("unable to correctly read the body of the request")
		s.sendError(standardErrMessage, w)
		return
	}

	// mutate the request
	mutated, err := s.Mutate(body)
	if err != nil {
		klog.Error(err)
		standardErrMessage := fmt.Errorf("unable to correctly mutate the request")
		s.sendError(standardErrMessage, w)
		return
	}

	// and write it back
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(mutated)

	if err := r.Body.Close(); err != nil {
		klog.Error("error in body closing")
	}
}

func (s *MutationServer) sendError(err error, w http.ResponseWriter) {
	klog.Error(err)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = fmt.Fprintf(w, "%s", err)
}

// Serve is a wrapper function for ListenAndServeTLS.
func (s *MutationServer) Serve() {
	if err := s.server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile); !errors.Is(err, http.ErrServerClosed) {
		// Error starting or closing listener:
		klog.Fatalf("HTTP server ListenAndServe: %v", err)
	}
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
func (s *MutationServer) Shutdown(ctx context.Context) {
	if err := s.server.Shutdown(ctx); err != nil {
		// Error from closing listeners, or context timeout:
		klog.Errorf("HTTP server Shutdown: %v", err)
	}
}
