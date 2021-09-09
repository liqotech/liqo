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

package crdreplicator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	netv1alpha1 "github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/clusterid"
	"github.com/liqotech/liqo/pkg/consts"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/testutil"
)

func TestPeeringPhase(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PeeringPhase")
}

const (
	timeout  = time.Second * 30
	interval = time.Millisecond * 250
)

var _ = Describe("PeeringPhase-Based Replication", func() {

	var (
		cluster    testutil.Cluster
		controller Controller
		mgr        manager.Manager
		ctx        context.Context
		cancel     context.CancelFunc
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		var err error
		cluster, mgr, err = testutil.NewTestCluster([]string{filepath.Join("..", "..", "deployments", "liqo", "crds")})
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}

		k8sclient = kubernetes.NewForConfigOrDie(mgr.GetConfig())

		tenantmanager := tenantnamespace.NewTenantNamespaceManager(k8sclient)
		clusterIDInterface := clusterid.NewStaticClusterID(localClusterID)

		dynClient := dynamic.NewForConfigOrDie(mgr.GetConfig())

		controller = Controller{
			Scheme:              mgr.GetScheme(),
			Client:              mgr.GetClient(),
			ClusterID:           localClusterID,
			RemoteDynClients:    map[string]dynamic.Interface{remoteClusterID: dynClient},
			RegisteredResources: nil,
			RemoteWatchers:      map[string]map[string]chan struct{}{},
			LocalDynClient:      dynClient,
			LocalWatchers:       map[string]chan struct{}{},

			NamespaceManager:                 tenantmanager,
			IdentityReader:                   identitymanager.NewCertificateIdentityReader(k8sclient, clusterIDInterface, tenantmanager),
			LocalToRemoteNamespaceMapper:     map[string]string{},
			RemoteToLocalNamespaceMapper:     map[string]string{},
			ClusterIDToLocalNamespaceMapper:  map[string]string{},
			ClusterIDToRemoteNamespaceMapper: map[string]string{},
		}

		go mgr.GetCache().Start(ctx)
	})

	AfterEach(func() {
		cancel()

		err := cluster.GetEnv().Stop()
		if err != nil {
			By(err.Error())
			os.Exit(1)
		}
	})

	Context("Outgoing Resource Replication", func() {

		type outgoingReplicationTestcase struct {
			resource            *unstructured.Unstructured
			registeredResources []Resource
			peeringPhases       map[string]consts.PeeringPhase
			expectedError       types.GomegaMatcher
		}

		DescribeTable("Filter resources to replicate to the remote cluster",
			func(c outgoingReplicationTestcase) {
				controller.RegisteredResources = c.registeredResources
				controller.peeringPhases = c.peeringPhases

				controller.AddedHandler(c.resource, gvr)

				_, err := controller.LocalDynClient.Resource(gvr).Namespace(testNamespace).
					Get(context.TODO(), c.resource.GetName(), metav1.GetOptions{})
				Expect(err).To(c.expectedError)
			},

			Entry("replicated resource", outgoingReplicationTestcase{
				resource: getObj(),
				registeredResources: []Resource{
					{
						GroupVersionResource: netv1alpha1.NetworkConfigGroupVersionResource,
						PeeringPhase:         consts.PeeringPhaseAuthenticated,
					},
				},
				peeringPhases: map[string]consts.PeeringPhase{
					remoteClusterID: consts.PeeringPhaseEstablished,
				},
				expectedError: BeNil(),
			}),

			Entry("not replicated resource (phase not enabled)", outgoingReplicationTestcase{
				resource: getObj(),
				registeredResources: []Resource{
					{
						GroupVersionResource: netv1alpha1.NetworkConfigGroupVersionResource,
						PeeringPhase:         consts.PeeringPhaseOutgoing,
					},
				},
				peeringPhases: map[string]consts.PeeringPhase{
					remoteClusterID: consts.PeeringPhaseIncoming,
				},
				expectedError: Not(BeNil()),
			}),

			Entry("not replicated resource (peering not established)", outgoingReplicationTestcase{
				resource: getObj(),
				registeredResources: []Resource{
					{
						GroupVersionResource: netv1alpha1.NetworkConfigGroupVersionResource,
						PeeringPhase:         consts.PeeringPhaseEstablished,
					},
				},
				peeringPhases: map[string]consts.PeeringPhase{
					remoteClusterID: consts.PeeringPhaseNone,
				},
				expectedError: Not(BeNil()),
			}),
		)

	})

	Context("Enable Outgoing Replication", func() {

		It("Enable Outgoing Replication", func() {

			gvr := discoveryv1alpha1.GroupVersion.WithResource("resourcerequests")
			remoteNamespace := "remote-1"

			controller.RegisteredResources = []Resource{
				{
					GroupVersionResource: gvr,
					PeeringPhase:         consts.PeeringPhaseEstablished,
				},
			}
			controller.peeringPhases = map[string]consts.PeeringPhase{
				remoteClusterID: consts.PeeringPhaseNone,
			}
			controller.ClusterIDToLocalNamespaceMapper[remoteClusterID] = testNamespace
			controller.LocalToRemoteNamespaceMapper[testNamespace] = remoteNamespace
			controller.ClusterIDToRemoteNamespaceMapper[remoteClusterID] = remoteNamespace

			// this namespace will be used as a remote cluster namespace
			_, err := k8sclient.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: remoteNamespace,
				},
			}, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			obj := getObjNamespaced()
			obj, err = controller.LocalDynClient.Resource(gvr).Namespace(testNamespace).
				Create(ctx, obj, metav1.CreateOptions{})
			Expect(err).To(BeNil())

			controller.checkResourcesOnPeeringPhaseChange(ctx, remoteClusterID,
				consts.PeeringPhaseNone, consts.PeeringPhaseNone)

			_, err = controller.LocalDynClient.Resource(gvr).Namespace(remoteNamespace).
				Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
			Expect(kerrors.IsNotFound(err)).To(BeTrue())

			// change peering phase
			controller.peeringPhases[remoteClusterID] = consts.PeeringPhaseOutgoing
			controller.checkResourcesOnPeeringPhaseChange(ctx, remoteClusterID,
				consts.PeeringPhaseOutgoing, consts.PeeringPhaseNone)

			Eventually(func() error {
				_, err = controller.LocalDynClient.Resource(gvr).Namespace(remoteNamespace).
					Get(context.TODO(), obj.GetName(), metav1.GetOptions{})
				return err
			}, timeout, interval).Should(BeNil())
		})

	})

})
