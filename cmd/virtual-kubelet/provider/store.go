// Copyright 2019-2021 The Liqo Authors
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

package provider

import (
	"sync"
	"time"

	"github.com/liqotech/liqo/internal/utils/errdefs"
	"github.com/liqotech/liqo/pkg/virtualKubelet/manager"
)

// Store is used for registering/fetching providers.
type Store struct {
	mu sync.Mutex
	ls map[string]InitFunc
}

// NewStore creates a new Store instance.
func NewStore() *Store {
	return &Store{
		ls: make(map[string]InitFunc),
	}
}

// Register registers a providers init func by name.
func (s *Store) Register(name string, f InitFunc) error {
	if f == nil {
		return errdefs.InvalidInput("provided init function cannot not be nil")
	}
	s.mu.Lock()
	s.ls[name] = f
	s.mu.Unlock()
	return nil
}

// Get gets the registered init func for the given name
// The returned function may be nil if the given name is not registered.
func (s *Store) Get(name string) InitFunc {
	s.mu.Lock()
	f := s.ls[name]
	s.mu.Unlock()
	return f
}

// List lists all the registered providers.
func (s *Store) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ls := make([]string, 0, len(s.ls))
	for p := range s.ls {
		ls = append(ls, p)
	}

	return ls
}

// Exists returns if there is an init function registered under the provided name.
func (s *Store) Exists(name string) bool {
	s.mu.Lock()
	_, ok := s.ls[name]
	s.mu.Unlock()
	return ok
}

// InitConfig is the config passed to initialize a registered provider.
type InitConfig struct {
	NodeName             string
	InternalIP           string
	DaemonPort           int32
	KubeClusterDomain    string
	ResourceManager      *manager.ResourceManager
	HomeKubeConfig       string
	RemoteKubeConfig     string
	HomeClusterID        string
	RemoteClusterID      string
	LiqoIpamServer       string
	InformerResyncPeriod time.Duration
}

// InitFunc defines the signature of the function creating a Provider instance based on the corresponding configuration.
type InitFunc func(InitConfig) (Provider, error)
