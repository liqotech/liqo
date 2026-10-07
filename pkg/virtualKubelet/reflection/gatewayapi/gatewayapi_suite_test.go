// Copyright 2019-2026 The Liqo Authors
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

package gatewayapi_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster-id"
	RemoteClusterID = "remote-cluster-id"

	LiqoNodeName = "liqo-remote-cluster"
	LiqoNodeIP   = "1.1.1.1"
)

var (
	testEnv    envtest.Environment
	restConfig *rest.Config
)

func TestGatewayAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gateway API Reflection Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
	forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)

	// The Gateway API CRDs are retrieved from the Go module, to match the version of the client.
	// The fake clients cannot be used for Gateways, since they incorrectly pluralize the resource name.
	output, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "sigs.k8s.io/gateway-api").Output()
	Expect(err).ToNot(HaveOccurred())
	crds := filepath.Join(strings.TrimSpace(string(output)), "config", "crd", "standard")

	testEnv = envtest.Environment{CRDInstallOptions: envtest.CRDInstallOptions{Paths: []string{
		filepath.Join(crds, "gateway.networking.k8s.io_gatewayclasses.yaml"),
		filepath.Join(crds, "gateway.networking.k8s.io_gateways.yaml"),
		filepath.Join(crds, "gateway.networking.k8s.io_httproutes.yaml"),
		filepath.Join(crds, "gateway.networking.k8s.io_grpcroutes.yaml"),
		filepath.Join(crds, "gateway.networking.k8s.io_referencegrants.yaml"),
		// TCPRoutes are not reflected, and they are used to test the generation of the corresponding events.
		filepath.Join(crds, "..", "experimental", "gateway.networking.k8s.io_tcproutes.yaml"),
		filepath.Join("..", "..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds", "offloading.liqo.io_shadowgatewaystatuses.yaml"),
		filepath.Join("..", "..", "..", "..", "deployments", "liqo", "charts", "liqo-crds", "crds", "offloading.liqo.io_shadowroutestatuses.yaml"),
	}, ErrorIfPathMissing: true}}
	restConfig, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	client := kubernetes.NewForConfigOrDie(restConfig)
	_, err = client.CoreV1().Nodes().Create(context.Background(), testutil.FakeNodeWithNameAndLabels(LiqoNodeName, nil), metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
	for _, namespace := range []string{LocalNamespace, RemoteNamespace} {
		_, err = client.CoreV1().Namespaces().Create(context.Background(),
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
		Expect(err).ToNot(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})
