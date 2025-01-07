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
//

package info

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
)

type dummyChecker struct {
	CheckerCommon
	title string
	id    string
	data  interface{}

	nCollectCalls int
}

func (d *dummyChecker) Collect(_ context.Context, _ Options) {
	d.nCollectCalls++
}

func (d *dummyChecker) Format(options Options) string {
	return ""
}

func (d *dummyChecker) GetData() interface{} {
	return d.data
}

// GetID returns the id of the section collected by the checker.
func (d *dummyChecker) GetID() string {
	if d.id != "" {
		return d.id
	}
	return "dummy"
}

// GetTitle returns the title of the section collected by the checker.
func (d *dummyChecker) GetTitle() string {
	if d.id != "" {
		return d.title
	}
	return "Dummy"
}

type dummyMultiClusterChecker struct {
	CheckerCommon
	title string
	id    string
	data  map[liqov1beta1.ClusterID]interface{}

	nCollectCalls int
}

func (d *dummyMultiClusterChecker) Collect(_ context.Context, _ Options) {
	d.nCollectCalls++
}

func (d *dummyMultiClusterChecker) FormatForClusterID(clusterID liqov1beta1.ClusterID, options Options) string {
	return ""
}

func (d *dummyMultiClusterChecker) GetDataByClusterID(clusterID liqov1beta1.ClusterID) (interface{}, error) {
	if res, ok := d.data[clusterID]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("no data collected for cluster %q", clusterID)
}

// GetID returns the id of the section collected by the checker.
func (d *dummyMultiClusterChecker) GetID() string {
	if d.id != "" {
		return d.id
	}
	return "dummy-multi-cluster"
}

// GetTitle returns the title of the section collected by the checker.
func (d *dummyMultiClusterChecker) GetTitle() string {
	if d.id != "" {
		return d.title
	}
	return "DummyMultiCluster"
}

func TestLocal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Local Suite")
}

var _ = BeforeSuite(func() {
	utilruntime.Must(liqov1beta1.AddToScheme(scheme.Scheme))
})
