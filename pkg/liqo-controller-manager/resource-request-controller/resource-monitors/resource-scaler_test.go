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

package resourcemonitors

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ ResourceReader = &FakeResourceReader{}

type FakeResourceReader struct {
	corev1.ResourceList
}

func (r FakeResourceReader) Register(context.Context, ResourceUpdateNotifier) {
}

func (r FakeResourceReader) ReadResources(context.Context, string) ([]*ResourceList, error) {
	resources := make([]*ResourceList, 1)
	resources[0] = &ResourceList{}
	resources[0].Resources = make(map[string]*resource.Quantity)
	resources[0].PoolName = "fake"
	for k, v := range r.ResourceList {
		vCopy := v.DeepCopy()
		resources[0].Resources[k.String()] = &vCopy
	}
	return resources, nil
}

func (r FakeResourceReader) RemoveClusterID(context.Context, string) error {
	return nil
}

var _ = Describe("ResourceMonitors Suite", func() {
	Context("ResourceScaler", func() {
		It("Scales resources correctly", func() {
			provider := FakeResourceReader{corev1.ResourceList{
				"cpu":    resource.MustParse("1000m"),
				"memory": resource.MustParse("8G"),
			}}
			scaler := ResourceScaler{
				Provider: provider,
				Factor:   .5,
			}
			scaled, _ := scaler.ReadResources(context.Background(), "")
			Expect(scaled).To(HaveLen(1))
			Expect(scaled[0].Resources).To(HaveLen(2))
			Expect(scaled[0].Resources["cpu"].Equal(resource.MustParse("500m"))).To(BeTrue())
			Expect(scaled[0].Resources["memory"].Equal(resource.MustParse("4G"))).To(BeTrue())
		})
	})
})
