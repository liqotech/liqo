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

package storage

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"k8s.io/utils/trace"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/manager"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/options"
)

const (
	LocalNamespace  = "local-namespace"
	RemoteNamespace = "remote-namespace"

	LocalClusterID  = "local-cluster-id"
	RemoteClusterID = "remote-cluster-id"

	VirtualStorageClassName    = "liqo"
	RealRemoteStorageClassName = "other-class"

	VirtualNodeName = "liqo-node"
	RealNodeName    = "real-node"
	localPvcName    = "pvc-local"
	remotePvcName   = "pvc-remote"
	remotePvcName2  = "pvc-remote-2"
)

var (
	testEnv   envtest.Environment
	k8sClient kubernetes.Interface

	ctx    context.Context
	cancel context.CancelFunc

	reflectorBuilder func(*options.NamespacedOpts) manager.NamespacedReflector
	factory          informers.SharedInformerFactory
	reflector        *NamespacedPersistentVolumeClaimReflector

	checkErrIgnoreAlreadyExists = func(_ runtime.Object, err error) {
		if !apierrors.IsAlreadyExists(err) {
			checkErr(nil, err)
		}
	}

	checkErr = func(_ runtime.Object, err error) {
		Expect(err).ToNot(HaveOccurred())
	}
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Reflection Suite")
}

var _ = BeforeSuite(func() {
	testutil.LogsToGinkgoWriter()

	testEnv = envtest.Environment{}
	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	// Need to use a real client, as server side apply seems not to be currently supported by the fake one.
	k8sClient = kubernetes.NewForConfigOrDie(cfg)

	_, err = k8sClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: LocalNamespace,
		},
	}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())

	_, err = k8sClient.CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: RemoteNamespace,
		},
	}, metav1.CreateOptions{})
	Expect(err).ToNot(HaveOccurred())
})

var _ = BeforeEach(func() {
	ctx, cancel = context.WithCancel(context.Background())
	ctx = trace.ContextWithTrace(ctx, trace.New("PersistentVolumeClaim"))

	bindingMode := storagev1.VolumeBindingWaitForFirstConsumer
	sc1 := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "liqo",
		},
		Provisioner:       consts.StorageProvisionerName,
		VolumeBindingMode: &bindingMode,
	}

	sc2 := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-class",
		},
		Provisioner:       consts.StorageProvisionerName,
		VolumeBindingMode: &bindingMode,
	}

	realNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: RealNodeName,
		},
	}

	virtualNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: VirtualNodeName,
			Labels: map[string]string{
				consts.RemoteClusterID: RemoteClusterID,
			},
		},
	}

	localPvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      localPvcName,
			Namespace: LocalNamespace,
			Labels: map[string]string{
				testutil.FakeNotReflectedLabelKey: "true",
			},
			Annotations: map[string]string{
				annStorageProvisioner:             consts.StorageProvisionerName,
				annSelectedNode:                   RealNodeName,
				testutil.FakeNotReflectedAnnotKey: "true",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: pointer.String(VirtualStorageClassName),
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	remotePvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      remotePvcName,
			Namespace: LocalNamespace,
			Annotations: map[string]string{
				annStorageProvisioner: consts.StorageProvisionerName,
				annSelectedNode:       VirtualNodeName,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: pointer.String(VirtualStorageClassName),
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	remotePvc2 := remotePvc.DeepCopy()
	remotePvc2.Name = remotePvcName2

	checkErrIgnoreAlreadyExists(k8sClient.CoreV1().Nodes().Create(ctx, realNode, metav1.CreateOptions{}))
	checkErrIgnoreAlreadyExists(k8sClient.CoreV1().Nodes().Create(ctx, virtualNode, metav1.CreateOptions{}))
	checkErrIgnoreAlreadyExists(k8sClient.StorageV1().StorageClasses().Create(ctx, sc1, metav1.CreateOptions{}))
	checkErrIgnoreAlreadyExists(k8sClient.StorageV1().StorageClasses().Create(ctx, sc2, metav1.CreateOptions{}))
	checkErrIgnoreAlreadyExists(k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Create(ctx, localPvc, metav1.CreateOptions{}))
	checkErrIgnoreAlreadyExists(k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Create(ctx, remotePvc, metav1.CreateOptions{}))
	checkErrIgnoreAlreadyExists(k8sClient.CoreV1().PersistentVolumeClaims(LocalNamespace).Create(ctx, remotePvc2, metav1.CreateOptions{}))

	forge.Init(LocalClusterID, RemoteClusterID, virtualNode.Name, "127.0.0.1")

	reflectorBuilder = NewNamespacedPersistentVolumeClaimReflector(VirtualStorageClassName,
		RealRemoteStorageClassName, true)
	factory = informers.NewSharedInformerFactory(k8sClient, 10*time.Hour)
})

var _ = JustBeforeEach(func() {
	options := options.NewNamespaced().
		WithLocal(LocalNamespace, k8sClient, factory).
		WithRemote(RemoteNamespace, k8sClient, factory).
		WithHandlerFactory(FakeEventHandler).WithEventBroadcaster(record.NewBroadcaster()).
		WithForgingOpts(testutil.FakeForgingOpts())

	reflector = reflectorBuilder(options).(*NamespacedPersistentVolumeClaimReflector)
	Expect(reflector).ToNot(BeNil())

	factory.Start(ctx.Done())
	factory.WaitForCacheSync(ctx.Done())
})

var _ = AfterEach(func() {
	cancel()
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

var FakeEventHandler = func(options.Keyer, ...options.EventFilter) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(_ interface{}) {},
		UpdateFunc: func(_, obj interface{}) {},
		DeleteFunc: func(_ interface{}) {},
	}
}
