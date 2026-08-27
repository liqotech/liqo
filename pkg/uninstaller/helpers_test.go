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

package uninstaller_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/uninstaller"
)

var _ = Describe("ForeignCluster annotation helper", func() {
	var (
		ctx context.Context
		cl  client.Client
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme := runtime.NewScheme()
		Expect(liqov1beta1.AddToScheme(scheme)).To(Succeed())
		cl = fake.NewClientBuilder().WithScheme(scheme).Build()
	})

	It("should annotate all ForeignClusters as permanently unreachable", func() {
		fc1 := &liqov1beta1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster-1"},
		}
		fc2 := &liqov1beta1.ForeignCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "cluster-2",
				Annotations: map[string]string{"other": "value"},
			},
		}
		Expect(cl.Create(ctx, fc1)).To(Succeed())
		Expect(cl.Create(ctx, fc2)).To(Succeed())

		Expect(uninstaller.MarkForeignClustersPermanentlyUnreachable(ctx, cl)).To(Succeed())

		var clusters liqov1beta1.ForeignClusterList
		Expect(cl.List(ctx, &clusters)).To(Succeed())
		Expect(clusters.Items).To(HaveLen(2))

		for _, fc := range clusters.Items {
			value, ok := fc.GetAnnotations()[consts.ForeignClusterPermanentlyUnreachableAnnotationKey]
			Expect(ok).To(BeTrue(), "ForeignCluster %q should have the permanently-unreachable annotation", fc.Name)
			Expect(value).To(Equal("true"))
		}
	})
})
