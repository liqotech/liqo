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

package tunneloperator

import (
	"fmt"
	"strings"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/liqotech/liqo/apis/net/v1alpha1"
	"github.com/liqotech/liqo/pkg/utils/slice"
)

const (
	oldIP1    = "10.0.1.2"
	oldIP2    = "10.0.1.5"
	newIP1    = "10.0.2.4"
	newIP2    = "10.0.3.7"
	DNAT      = "DNAT"
	timeout   = time.Second * 10
	interval  = time.Millisecond * 250
	namespace = "test"
)

var _ = Describe("NatmappingOperator", func() {
	BeforeEach(func() {
		request.Namespace = namespace
		// Names to resources will be given in Its
		nm1 = &v1alpha1.NatMapping{
			ObjectMeta: v1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1alpha1.NatMappingSpec{
				ClusterID:       clusterID1,
				ClusterMappings: make(v1alpha1.Mappings),
			},
		}
		nm2 = &v1alpha1.NatMapping{
			ObjectMeta: v1.ObjectMeta{
				Namespace: namespace,
			},
			Spec: v1alpha1.NatMappingSpec{
				ClusterID:       clusterID2,
				ClusterMappings: make(v1alpha1.Mappings),
			},
		}
	})
	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &v1alpha1.NatMapping{}, &client.DeleteAllOfOptions{
			ListOptions: client.ListOptions{
				Namespace: namespace,
			},
		})
		Expect(err).To(BeNil())
	})
	Context("Reconcile on a resource that does not exist", func() {
		It("the controller should return no errors", func() {
			nm2.ObjectMeta.Name = "resource-not-exists"
			request.Name = nm2.ObjectMeta.Name
			Consistently(func() error { _, err := controller.Reconcile(ctx, request); return err }).Should(BeNil())
		})
	})
	Context("If the cluster is not ready", func() {
		It("the controller should do nothing", func() {
			nm2.ObjectMeta.Name = "cluster-not-ready"
			request.Name = nm2.ObjectMeta.Name
			// ClusterID2 is set as non ready in BeforeSuite,
			// then even if natmapping resource of cluster
			// contains some mappings, they should not result
			// in real IPTables rules.

			// Populate resource
			nm2.Spec.ClusterMappings[oldIP1] = newIP1
			nm2.Spec.ClusterMappings[oldIP2] = newIP2
			Eventually(func() error {
				err := k8sClient.Create(ctx, nm2, &client.CreateOptions{})
				return err
			}).Should(BeNil())

			Eventually(func() error { _, err := controller.Reconcile(ctx, request); return err }).
				Should(MatchError(fmt.Sprintf("tunnel for cluster {%s} is not ready", clusterID2)))

			Consistently(func() []string {
				// Rules should not be present
				rules, err := ListRulesInChainInCustomNs(getClusterPreRoutingMappingChain(clusterID1))
				if err != nil {
					Fail(fmt.Sprintf("failed to list rules in chain %s: %s", getClusterPreRoutingMappingChain(clusterID1), err))
				}
				return rules
			}).Should(BeEmpty())
		})
	})
	Context("If the cluster is ready", func() {
		It("the controller should insert the right rules", func() {
			nm1.ObjectMeta.Name = "cluster-ready"
			request.Name = nm1.ObjectMeta.Name
			// ClusterID1 is set as ready in BeforeSuite

			// Populate resource
			nm1.Spec.ClusterMappings[oldIP1] = newIP1
			nm1.Spec.ClusterMappings[oldIP2] = newIP2
			Eventually(func() error {
				err := k8sClient.Create(ctx, nm1, &client.CreateOptions{})
				return err
			}).Should(BeNil())
			Consistently(func() error { _, err := controller.Reconcile(ctx, request); return err }).Should(BeNil())
			Eventually(func() []string {
				rules, err := ListRulesInChainInCustomNs(getClusterPreRoutingMappingChain(clusterID1))
				if err != nil {
					Fail(fmt.Sprintf("failed to list rules in chain %s: %s", getClusterPreRoutingMappingChain(clusterID1), err))
				}
				return rules
			}, timeout, interval).Should(ContainElements(
				fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1),
				fmt.Sprintf("-d %s -j %s --to-destination %s", newIP2, DNAT, oldIP2),
			))
		})
	})
	Context("If natmapping resource is updated with deletion and insertion of mappings", func() {
		It("the controller should update IPTables rules", func() {
			nm1.ObjectMeta.Name = "update"
			request.Name = nm1.ObjectMeta.Name
			// Insert mapping for oldIP1
			nm1.Spec.ClusterMappings[oldIP1] = newIP1
			Eventually(func() error {
				err := k8sClient.Create(ctx, nm1, &client.CreateOptions{})
				return err
			}).Should(BeNil())

			Consistently(func() error { _, err := controller.Reconcile(ctx, request); return err }).Should(BeNil())

			Eventually(func() []string {
				rules, err := ListRulesInChainInCustomNs(getClusterPreRoutingMappingChain(clusterID1))
				if err != nil {
					Fail(fmt.Sprintf("failed to list rules in chain %s: %s", getClusterPreRoutingMappingChain(clusterID1), err))
				}
				return rules
			}, timeout, interval).Should(ContainElements(
				fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1),
			))

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: nm1.ObjectMeta.Name, Namespace: namespace}, nm1)
				if err != nil {
					return false
				}
				// Delete mapping for oldIP1
				delete(nm1.Spec.ClusterMappings, oldIP1)
				// Insert mapping for oldIP2
				nm1.Spec.ClusterMappings[oldIP2] = newIP2
				err = k8sClient.Update(ctx, nm1, &client.UpdateOptions{})
				return err == nil
			}).Should(BeTrue())

			Consistently(func() error { _, err := controller.Reconcile(ctx, request); return err }).Should(BeNil())

			Eventually(func() bool {
				rules, err := ListRulesInChainInCustomNs(getClusterPreRoutingMappingChain(clusterID1))
				if err != nil {
					Fail(fmt.Sprintf("failed to list rules in chain %s: %s", getClusterPreRoutingMappingChain(clusterID1), err))
				}
				// Should contain the rule for oldIP1 but it should not contain the rule for oldIP2
				if slice.ContainsString(rules, fmt.Sprintf("-d %s -j %s --to-destination %s", newIP2, DNAT, oldIP2)) &&
					!slice.ContainsString(rules, fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1)) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("If mappings for more clusters are active", func() {
		It("the controller should update IPTables rules independently", func() {
			nm1.ObjectMeta.Name = "more-clusters-1"
			request.Name = nm1.ObjectMeta.Name
			// Set cluster2 as ready
			readyClustersMutex.Lock()
			readyClusters[clusterID2] = struct{}{}
			readyClustersMutex.Unlock()
			// Insert mapping for oldIP1 in cluster1
			nm1.Spec.ClusterMappings[oldIP1] = newIP1
			Eventually(func() error {
				err := k8sClient.Create(ctx, nm1, &client.CreateOptions{})
				return err
			}).Should(BeNil())

			Consistently(func() error { _, err := controller.Reconcile(ctx, request); return err }).Should(BeNil())

			// Insert mapping for oldIP2 in cluster2
			nm2.ObjectMeta.Name = "more-clusters-2"
			request.Name = nm2.ObjectMeta.Name
			nm2.Spec.ClusterMappings[oldIP2] = newIP2
			Eventually(func() error {
				err := k8sClient.Create(ctx, nm2, &client.CreateOptions{})
				return err
			}).Should(BeNil())

			Consistently(func() error { _, err := controller.Reconcile(ctx, request); return err }).Should(BeNil())

			Eventually(func() bool {
				cluster1Rules, err := ListRulesInChainInCustomNs(getClusterPreRoutingMappingChain(clusterID1))
				if err != nil {
					Fail(fmt.Sprintf("failed to list rules in chain %s: %s", getClusterPreRoutingMappingChain(clusterID1), err))
				}
				// Cluster1 rules should contain the rule for oldIP1 and not contain the rule for oldIP2
				if !slice.ContainsString(cluster1Rules, fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1)) ||
					slice.ContainsString(cluster1Rules, fmt.Sprintf("-d %s -j %s --to-destination %s", newIP2, DNAT, oldIP2)) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())

			Eventually(func() bool {
				cluster2Rules, err := ListRulesInChainInCustomNs(getClusterPreRoutingMappingChain(clusterID2))
				if err != nil {
					Fail(fmt.Sprintf("failed to list rules in chain %s: %s", getClusterPreRoutingMappingChain(clusterID2), err))
				}
				// Cluster2 rules should contain the rule for oldIP2 and not contain the rule for oldIP1
				if !slice.ContainsString(cluster2Rules, fmt.Sprintf("-d %s -j %s --to-destination %s", newIP2, DNAT, oldIP2)) ||
					slice.ContainsString(cluster2Rules, fmt.Sprintf("-d %s -j %s --to-destination %s", newIP1, DNAT, oldIP1)) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})
})

func getClusterPreRoutingMappingChain(clusterID string) string {
	const liqonetPreRoutingMappingClusterChainPrefix = "LIQO-PRRT-MAP-CLS-"
	return fmt.Sprintf("%s%s", liqonetPreRoutingMappingClusterChainPrefix, strings.Split(clusterID, "-")[0])
}

func ListRulesInChainInCustomNs(chain string) ([]string, error) {
	var rules []string
	var err error
	err = iptNetns.Do(func(nn ns.NetNS) error {
		rules, err = ipt.ListRulesInChain(chain)
		return err
	})
	if err != nil {
		return nil, err
	}
	return rules, nil
}
