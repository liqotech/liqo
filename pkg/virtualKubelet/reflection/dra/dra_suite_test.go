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

package dra_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster-id"
	RemoteClusterID = "remote-cluster-id"

	LiqoNodeName = "local-node"
	LiqoNodeIP   = "1.1.1.1"
	LiqoNodeUID  = types.UID("liqo-node-uid")

	// Shared test constants used across DRA test files.
	barVal         = "bar"
	fooVal         = "foo"
	trueVal        = "true"
	fakeClaimName  = "req-1"
	fakeDriverName = "test.driver"
	fakePoolName   = "pool"
)

var (
	// Per-test fake clients. Recreated in BeforeEach so tests are isolated.
	localClient, remoteClient *fake.Clientset

	ctx    context.Context
	cancel context.CancelFunc
)

func TestDRA(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DRA Reflection Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()
	forge.Init(LocalClusterID, RemoteClusterID, LiqoNodeName, LiqoNodeIP)
})

var _ = BeforeEach(func() {
	ctx, cancel = context.WithCancel(context.Background())

	// Local cluster preloaded with the virtual node so OwnerReference resolution
	// works without each test having to recreate it.
	localClient = fake.NewClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: LiqoNodeName,
			UID:  LiqoNodeUID,
			Labels: map[string]string{
				corev1.LabelHostname: LiqoNodeName,
				consts.TypeLabel:     consts.TypeNode,
			},
		},
	})
	remoteClient = fake.NewClientset()
})

var _ = AfterEach(func() { cancel() })

// FakeEventHandler is a no-op handler factory used to bypass real informer
// event-handler wiring in tests.
var FakeEventHandler = func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, _ interface{}) {},
		DeleteFunc: func(_ interface{}) {},
	}
}
