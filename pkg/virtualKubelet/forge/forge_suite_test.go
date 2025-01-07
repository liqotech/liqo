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

package forge_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const (
	LocalClusterID  liqov1beta1.ClusterID = "local-cluster-id"
	RemoteClusterID liqov1beta1.ClusterID = "remote-cluster-id"
	LiqoNodeName                          = "local-node"
	LiqoNodeIP                            = "1.1.1.1"
)

func TestForge(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Forge Suite")
}

var _ = BeforeEach(func() {
	Expect(os.Setenv("KUBERNETES_SERVICE_PORT", "8443")).To(Succeed())

	forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)
})
