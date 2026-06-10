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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	resourcev1 "k8s.io/api/resource/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/liqotech/liqo/pkg/utils/testutil"
	"github.com/liqotech/liqo/pkg/virtualKubelet/forge"
	"github.com/liqotech/liqo/pkg/virtualKubelet/reflection/dra"
)

var _ = Describe("ensureRemoteDeviceClass", func() {
	const (
		className           = "gpu-class"
		deviceClassResource = "deviceclasses"
	)

	var (
		localClient, remoteClient *fake.Clientset
		opts                      *forge.ForgingOpts
	)

	BeforeEach(func() {
		localClient = fake.NewClientset()
		remoteClient = fake.NewClientset()
		opts = testutil.FakeForgingOpts()
	})

	ensureDeviceClass := func() error {
		return dra.EnsureRemoteDeviceClass(ctx, className,
			localClient.ResourceV1().DeviceClasses(),
			remoteClient.ResourceV1().DeviceClasses(),
			opts.LabelsNotReflected, opts.AnnotationsNotReflected)
	}

	When("the remote DeviceClass already exists", func() {
		var createCount int

		BeforeEach(func() {
			_, err := remoteClient.ResourceV1().DeviceClasses().Create(ctx, &resourcev1.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{Name: className},
			}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			createCount = 0
			remoteClient.PrependReactor("create", deviceClassResource,
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					createCount++
					return false, nil, nil
				},
			)
		})

		It("should return nil and not issue any Create on the remote", func() {
			Expect(ensureDeviceClass()).To(Succeed())
			Expect(createCount).To(Equal(0))
		})
	})

	When("the remote is missing and the local DeviceClass exists", func() {
		BeforeEach(func() {
			_, err := localClient.ResourceV1().DeviceClasses().Create(ctx, &resourcev1.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: className,
					Labels: map[string]string{
						fooVal:                            barVal,
						testutil.FakeNotReflectedLabelKey: trueVal,
					},
					Annotations: map[string]string{
						testutil.FakeNotReflectedAnnotKey: trueVal,
					},
				},
			}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create the remote with reflection labels and filtered keys", func() {
			Expect(ensureDeviceClass()).To(Succeed())

			remote, err := remoteClient.ResourceV1().DeviceClasses().Get(ctx, className, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(remote.Labels).To(HaveKeyWithValue(fooVal, barVal))
			Expect(remote.Labels).To(HaveKeyWithValue(forge.LiqoOriginClusterIDKey, LocalClusterID))
			Expect(remote.Labels).To(HaveKeyWithValue(forge.LiqoDestinationClusterIDKey, RemoteClusterID))
			Expect(remote.Labels).ToNot(HaveKey(testutil.FakeNotReflectedLabelKey))
			Expect(remote.Annotations).ToNot(HaveKey(testutil.FakeNotReflectedAnnotKey))
		})
	})

	When("both remote and local are missing", func() {
		It("should return a NotFound error mentioning that the local class does not exist", func() {
			err := ensureDeviceClass()
			Expect(err).To(HaveOccurred())
			Expect(kerrors.IsNotFound(err)).To(BeTrue(),
				"the error chain must surface NotFound so callers can soft-skip via kerrors.IsNotFound")
			Expect(err.Error()).To(ContainSubstring("cannot reflect DeviceClass"))
		})
	})

	When("the remote Get returns a transient error", func() {
		BeforeEach(func() {
			remoteClient.PrependReactor("get", deviceClassResource,
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("remote boom")
				},
			)
		})
		It("should propagate the error wrapped with a 'getting remote DeviceClass' prefix", func() {
			err := ensureDeviceClass()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("getting remote DeviceClass"))
			Expect(err.Error()).To(ContainSubstring("remote boom"))
		})
	})

	When("the local Get returns a transient error", func() {
		BeforeEach(func() {
			// Remote not present so the function moves on to fetch the local.
			localClient.PrependReactor("get", deviceClassResource,
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("local boom")
				},
			)
		})
		It("should propagate the error wrapped with a 'getting local DeviceClass' prefix", func() {
			err := ensureDeviceClass()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("getting local DeviceClass"))
			Expect(err.Error()).To(ContainSubstring("local boom"))
		})
	})

	When("the remote Create returns AlreadyExists (concurrent create)", func() {
		BeforeEach(func() {
			_, err := localClient.ResourceV1().DeviceClasses().Create(ctx, &resourcev1.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{Name: className},
			}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			remoteClient.PrependReactor("create", deviceClassResource,
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, kerrors.NewAlreadyExists(schema.GroupResource{
						Group: "resource.k8s.io", Resource: deviceClassResource,
					}, className)
				},
			)
		})
		It("should swallow the error and report success", func() {
			Expect(ensureDeviceClass()).To(Succeed())
		})
	})

	When("the remote Create returns an unexpected error", func() {
		BeforeEach(func() {
			_, err := localClient.ResourceV1().DeviceClasses().Create(ctx, &resourcev1.DeviceClass{
				ObjectMeta: metav1.ObjectMeta{Name: className},
			}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())

			remoteClient.PrependReactor("create", deviceClassResource,
				func(_ clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("boom while creating")
				},
			)
		})
		It("should propagate the error wrapped with a 'creating remote DeviceClass' prefix", func() {
			err := ensureDeviceClass()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creating remote DeviceClass"))
			Expect(err.Error()).To(ContainSubstring("boom while creating"))
		})
	})
})
