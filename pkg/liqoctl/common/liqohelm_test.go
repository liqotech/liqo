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

package common

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LiqoHelm", func() {
	var (
		lhc                         *LiqoHelmClient
		expectedRelease             = "liqo"
		expectedClusterLabels       = map[string]interface{}{"label1": "value1", "label2": "value2", "label3": "value3"}
		expectedClusterLabelsString = map[string]string{"label1": "value1", "label2": "value2", "label3": "value3"}
		clusterLabels               map[string]string
	)
	Context("Creating a new LiqoHelmClient", func() {
		BeforeEach(func() {
			lhc = &LiqoHelmClient{
				release: expectedRelease,
				getValues: func() (map[string]interface{}, error) {
					return map[string]interface{}{
						"discovery": map[string]interface{}{
							"config": map[string]interface{}{
								"clusterLabels": expectedClusterLabels,
							},
						},
					}, nil
				},
			}
		})
		JustBeforeEach(func() {
			clusterLabels, _ = lhc.GetClusterLabels()
		})
		It("should contain a valid release", func() {
			Expect(expectedRelease).To(Equal(lhc.release))
		})
		It("should return cluster labels", func() {
			Expect(expectedClusterLabelsString).To(Equal(clusterLabels))
		})
	})
})
