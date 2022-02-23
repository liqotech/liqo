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

package crdreplicator_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	crdreplicator "github.com/liqotech/liqo/internal/crdReplicator"
	"github.com/liqotech/liqo/internal/crdReplicator/reflection"
	"github.com/liqotech/liqo/internal/crdReplicator/resources"
	"github.com/liqotech/liqo/pkg/identityManager/fake"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestCrdReplicator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CRD Replicator Suite")
}

const (
	localNamespace  = "local-namespace"
	remoteNamespace = "remote-namespace"
)

var (
	ctx    context.Context
	cancel context.CancelFunc

	localCluster  discoveryv1alpha1.ClusterIdentity
	remoteCluster discoveryv1alpha1.ClusterIdentity
	cluster       testutil.Cluster
	controller    crdreplicator.Controller
	cl            client.Client
)

var _ = BeforeEach(func() {
	localCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "local-cluster-id",
		ClusterName: "local-cluster",
	}
	remoteCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "remote-cluster-id",
		ClusterName: "remote-cluster",
	}
})

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()

	SetDefaultEventuallyTimeout(2 * time.Second)
	SetDefaultConsistentlyDuration(time.Second)

	ctx, cancel = context.WithCancel(context.Background())

	// The same cluster is used both as local and remote, using different namespaces.
	clsrt, mgr, err := testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
	Expect(err).ToNot(HaveOccurred())
	cluster = clsrt

	cl, err = client.New(cluster.GetCfg(), client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	dynClient := dynamic.NewForConfigOrDie(cluster.GetCfg())

	localCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "local-cluster-id",
		ClusterName: "local-cluster",
	}
	remoteCluster = discoveryv1alpha1.ClusterIdentity{
		ClusterID:   "remote-cluster-id",
		ClusterName: "remote-cluster",
	}

	reflectionManager := reflection.NewManager(dynClient, localCluster.ClusterID, 1, 0)
	reflectionManager.Start(ctx, resources.GetResourcesToReplicate())

	controller = crdreplicator.Controller{
		Scheme:    mgr.GetScheme(),
		Client:    cl,
		ClusterID: localCluster.ClusterID,

		RegisteredResources: resources.GetResourcesToReplicate(),
		ReflectionManager:   reflectionManager,
		Reflectors:          make(map[string]*reflection.Reflector),

		IdentityReader: fake.NewIdentityReader().Add(remoteCluster.ClusterID, remoteNamespace, cluster.GetCfg()),
	}
	Expect(err).ToNot(HaveOccurred())
	Expect(controller.SetupWithManager(mgr)).To(Succeed())

	Expect(cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: localNamespace}}))
	Expect(cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: remoteNamespace}}))

	go func() {
		err = mgr.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	cancel()
	Expect(cluster.GetEnv().Stop()).To(Succeed())
})
