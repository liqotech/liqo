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
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	idManTest "github.com/liqotech/liqo/pkg/identityManager/testUtils"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/apiserver"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestAuthService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AuthService Suite")
}

var (
	cluster         testutil.Cluster
	clusterIdentity discoveryv1alpha1.ClusterIdentity
	authService     Controller

	ctx    context.Context
	cancel context.CancelFunc

	tMan tokenManagerMock

	csr []byte
)

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	testutil.LogsToGinkgoWriter()

	_ = tMan.createToken()

	var err error
	cluster, _, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
	Expect(err).ToNot(HaveOccurred())

	informerFactory := informers.NewSharedInformerFactoryWithOptions(cluster.GetClient(), 300*time.Second, informers.WithNamespace("default"))

	secretInformer := informerFactory.Core().V1().Secrets().Informer()
	secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{})

	clusterIdentity = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "default-cluster-id",
		ClusterName: "default-cluster-name",
	}

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	namespaceManager := tenantnamespace.NewManager(cluster.GetClient())
	identityProvider := identitymanager.NewCertificateIdentityProvider(
		ctx, cluster.GetClient(), clusterIdentity, namespaceManager)

	config := apiserver.Config{Address: cluster.GetCfg().Host, TrustedCA: false}
	Expect(config.Complete(cluster.GetCfg(), cluster.GetClient())).To(Succeed())

	authService = Controller{
		namespace:            "default",
		clientset:            cluster.GetClient(),
		secretInformer:       secretInformer,
		localCluster:         clusterIdentity,
		namespaceManager:     namespaceManager,
		identityProvider:     identityProvider,
		credentialsValidator: &tokenValidator{},
		apiServerConfig:      config,
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	_, err = cluster.GetClient().RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())

	idManTest.StartTestApprover(cluster.GetClient(), ctx.Done())
})

var _ = AfterSuite(func() {
	cancel()
	Expect(cluster.GetEnv().Stop()).To(Succeed())
})
