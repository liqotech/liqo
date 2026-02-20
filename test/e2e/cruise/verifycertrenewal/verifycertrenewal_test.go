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

package verifycertrenewal

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/test/e2e/testutils/config"
	"github.com/liqotech/liqo/test/e2e/testutils/tester"
	"github.com/liqotech/liqo/test/e2e/testutils/util"
)

const (
	// clustersRequired is the number of clusters required in this E2E test.
	clustersRequired = 2
	// testName is the name of this E2E test.
	testName = "VERIFY_CERT_RENEWAL"
)

func TestE2E(t *testing.T) {
	util.CheckIfTestIsSkipped(t, clustersRequired, testName)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Liqo E2E Suite")
}

var (
	ctx         = context.Background()
	testContext = tester.GetTester(ctx)
	interval    = config.Interval
	// Use a longer timeout since a certificate renewal might be in progress.
	timeout = 2 * time.Minute
)

var _ = Describe("Liqo E2E", func() {
	Context("Verify Certificate Renewal", func() {
		It("should have matching AuthParams between Identities and their corresponding resources", func() {
			for clusterIdx := range testContext.Clusters {
				cluster := &testContext.Clusters[clusterIdx]

				By(fmt.Sprintf("Listing identities on cluster %d (%s)", clusterIdx, cluster.Cluster))

				var identityList authv1beta1.IdentityList
				Eventually(func() error {
					return cluster.ControllerClient.List(ctx, &identityList)
				}, timeout, interval).Should(Succeed())

				for i := range identityList.Items {
					identity := &identityList.Items[i]

					// Fail fast: a certificate should never be expired.
					if len(identity.Spec.AuthParams.SignedCRT) > 0 {
						Expect(checkCertificateNotExpired(identity.Spec.AuthParams.SignedCRT,
							fmt.Sprintf("Identity %s/%s", identity.Namespace, identity.Name))).To(Succeed())
					}

					switch identity.Spec.Type {
					case authv1beta1.ResourceSliceIdentityType:
						verifyResourceSliceIdentity(cluster, identity)
					case authv1beta1.ControlPlaneIdentityType:
						verifyControlPlaneIdentity(cluster, identity)
					}
				}
			}
		})
	})
})

// verifyResourceSliceIdentity checks that the AuthParams in the Identity match the AuthParams
// in the corresponding ResourceSlice on the remote (provider) cluster.
func verifyResourceSliceIdentity(cluster *tester.ClusterContext, identity *authv1beta1.Identity) {
	rsName, ok := identity.Labels[consts.ResourceSliceNameLabelKey]
	Expect(ok).To(BeTrue(), "Identity %s/%s should have label %s",
		identity.Namespace, identity.Name, consts.ResourceSliceNameLabelKey)

	// Find the provider cluster matching the identity's ClusterID.
	providerCluster := findCluster(identity.Spec.ClusterID)
	Expect(providerCluster).ToNot(BeNil(),
		"Provider cluster %s not found in test context for Identity %s/%s",
		identity.Spec.ClusterID, identity.Namespace, identity.Name)

	By(fmt.Sprintf("Verifying ResourceSlice identity %s/%s matches ResourceSlice %s on provider %s",
		identity.Namespace, identity.Name, rsName, providerCluster.Cluster))

	Eventually(func() error {
		// Re-fetch the identity to get the latest version.
		var currentIdentity authv1beta1.Identity
		if err := cluster.ControllerClient.Get(ctx, client.ObjectKeyFromObject(identity), &currentIdentity); err != nil {
			return fmt.Errorf("failed to get Identity %s/%s: %w", identity.Namespace, identity.Name, err)
		}

		// Find the tenant namespace on the provider cluster for the consumer's cluster ID.
		var tenantList authv1beta1.TenantList
		if err := providerCluster.ControllerClient.List(ctx, &tenantList, client.MatchingLabels{
			consts.RemoteClusterID: string(cluster.Cluster),
		}); err != nil {
			return fmt.Errorf("failed to list Tenants on provider cluster %s: %w", providerCluster.Cluster, err)
		}

		if len(tenantList.Items) == 0 {
			return fmt.Errorf("no Tenant found for cluster %s on provider %s", cluster.Cluster, providerCluster.Cluster)
		}

		tenantNamespace := tenantList.Items[0].Status.TenantNamespace
		if tenantNamespace == "" {
			return fmt.Errorf("Tenant %s/%s has empty TenantNamespace", tenantList.Items[0].Namespace, tenantList.Items[0].Name)
		}

		// Get the ResourceSlice on the provider cluster in the tenant namespace.
		var rs authv1beta1.ResourceSlice
		if err := providerCluster.ControllerClient.Get(ctx, client.ObjectKey{
			Namespace: tenantNamespace,
			Name:      rsName,
		}, &rs); err != nil {
			return fmt.Errorf("failed to get ResourceSlice %s/%s on provider %s: %w",
				tenantNamespace, rsName, providerCluster.Cluster, err)
		}

		if rs.Status.AuthParams == nil {
			return fmt.Errorf("ResourceSlice %s/%s on provider %s has nil AuthParams",
				tenantNamespace, rsName, providerCluster.Cluster)
		}

		return compareAuthParams(&currentIdentity.Spec.AuthParams, rs.Status.AuthParams,
			fmt.Sprintf("Identity %s/%s vs ResourceSlice %s/%s (provider %s)",
				identity.Namespace, identity.Name, tenantNamespace, rsName, providerCluster.Cluster))
	}, timeout, interval).Should(Succeed())
}

// verifyControlPlaneIdentity checks that the AuthParams in the Identity match the AuthParams
// in the corresponding Tenant on the remote (provider) cluster.
func verifyControlPlaneIdentity(cluster *tester.ClusterContext, identity *authv1beta1.Identity) {
	By(fmt.Sprintf("Verifying ControlPlane identity %s/%s for remote cluster %s",
		identity.Namespace, identity.Name, identity.Spec.ClusterID))

	// Find the provider cluster matching the identity's ClusterID.
	providerCluster := findCluster(identity.Spec.ClusterID)
	Expect(providerCluster).ToNot(BeNil(),
		"Provider cluster %s not found in test context for Identity %s/%s",
		identity.Spec.ClusterID, identity.Namespace, identity.Name)

	Eventually(func() error {
		// Re-fetch the identity to get the latest version.
		var currentIdentity authv1beta1.Identity
		if err := cluster.ControllerClient.Get(ctx, client.ObjectKeyFromObject(identity), &currentIdentity); err != nil {
			return fmt.Errorf("failed to get Identity %s/%s: %w", identity.Namespace, identity.Name, err)
		}

		// Find the Tenant on the provider cluster for the consumer's cluster ID.
		var tenantList authv1beta1.TenantList
		if err := providerCluster.ControllerClient.List(ctx, &tenantList, client.MatchingLabels{
			consts.RemoteClusterID: string(cluster.Cluster),
		}); err != nil {
			return fmt.Errorf("failed to list Tenants on provider cluster %s: %w", providerCluster.Cluster, err)
		}

		if len(tenantList.Items) == 0 {
			return fmt.Errorf("no Tenant found for cluster %s on provider %s", cluster.Cluster, providerCluster.Cluster)
		}

		tenant := &tenantList.Items[0]
		if tenant.Status.AuthParams == nil {
			return fmt.Errorf("Tenant %s/%s has nil AuthParams", tenant.Namespace, tenant.Name)
		}

		return compareAuthParams(&currentIdentity.Spec.AuthParams, tenant.Status.AuthParams,
			fmt.Sprintf("Identity %s/%s vs Tenant %s/%s", identity.Namespace, identity.Name, tenant.Namespace, tenant.Name))
	}, timeout, interval).Should(Succeed())
}

// findCluster returns the ClusterContext for the given cluster ID, or nil if not found.
func findCluster(clusterID liqov1beta1.ClusterID) *tester.ClusterContext {
	for i := range testContext.Clusters {
		if testContext.Clusters[i].Cluster == clusterID {
			return &testContext.Clusters[i]
		}
	}
	return nil
}

// checkCertificateNotExpired parses the PEM-encoded certificate and returns an error if it is expired.
// If the certificate is empty (no SignedCRT yet), it returns an error indicating the certificate is pending.
func checkCertificateNotExpired(pemCert []byte, desc string) error {
	if len(pemCert) == 0 {
		return fmt.Errorf("%s: SignedCRT is empty, certificate not yet available", desc)
	}

	block, _ := pem.Decode(pemCert)
	if block == nil {
		return fmt.Errorf("%s: failed to decode PEM block", desc)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("%s: failed to parse certificate: %w", desc, err)
	}

	if time.Now().After(cert.NotAfter) {
		return fmt.Errorf("%s: certificate expired at %v", desc, cert.NotAfter)
	}

	return nil
}

// compareAuthParams compares two AuthParams and returns an error describing the first mismatch found.
func compareAuthParams(identityParams, resourceParams *authv1beta1.AuthParams, desc string) error {
	if identityParams.APIServer != resourceParams.APIServer {
		return fmt.Errorf("%s: APIServer mismatch: %q != %q", desc, identityParams.APIServer, resourceParams.APIServer)
	}

	if !bytes.Equal(identityParams.CA, resourceParams.CA) {
		return fmt.Errorf("%s: CA mismatch", desc)
	}

	if !bytes.Equal(identityParams.SignedCRT, resourceParams.SignedCRT) {
		return fmt.Errorf("%s: SignedCRT mismatch", desc)
	}

	if (identityParams.ProxyURL == nil) != (resourceParams.ProxyURL == nil) {
		return fmt.Errorf("%s: ProxyURL presence mismatch: identity=%v resource=%v",
			desc, identityParams.ProxyURL, resourceParams.ProxyURL)
	}
	if identityParams.ProxyURL != nil && *identityParams.ProxyURL != *resourceParams.ProxyURL {
		return fmt.Errorf("%s: ProxyURL mismatch: %q != %q", desc, *identityParams.ProxyURL, *resourceParams.ProxyURL)
	}

	return nil
}
