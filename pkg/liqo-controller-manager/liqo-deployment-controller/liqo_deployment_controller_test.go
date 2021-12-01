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

package liqodeploymentctrl_test

import (
	"bytes"
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	liqodeploymentctrl "github.com/liqotech/liqo/pkg/liqo-controller-manager/liqo-deployment-controller"
)

var _ = Describe("Reconcile", func() {
	const (
		liqoDeploymentNamespace string = "default"
		liqoDeploymentName      string = "test-liqo-deployment"
	)

	var (
		ctx    context.Context
		buffer *bytes.Buffer
		res    ctrl.Result
		err    error

		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: liqoDeploymentNamespace,
				Name:      liqoDeploymentName,
			},
		}
	)

	BeforeEach(func() {
		ctx = context.TODO()
		buffer = &bytes.Buffer{}
		klog.SetOutput(buffer)
	})

	JustBeforeEach(func() {
		r := &liqodeploymentctrl.Reconciler{
			Client: k8sClient,
			Scheme: scheme.Scheme,
		}

		res, err = r.Reconcile(ctx, req)
		klog.Flush()
	})

	When("liqodeployment is not found", func() {
		It("should ignore it", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(res).To(BeZero())
			Expect(buffer.String()).To(ContainSubstring(fmt.Sprintf("skip: liqodeployment %v not found", req.NamespacedName)))
		})
	})
})
