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

package identitymanager

import (
	"context"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	"github.com/liqotech/liqo/pkg/utils/csr"
)

var _ IdentityManager = &identityManager{}

type identityManager struct {
	IdentityProvider

	client           client.Client
	k8sClient        kubernetes.Interface
	localCluster     liqov1beta1.ClusterID
	namespaceManager tenantnamespace.Manager

	iamTokenManager tokenManager
}

// NewCertificateIdentityReader gets a new certificate identity reader.
func NewCertificateIdentityReader(ctx context.Context, cl client.Client, k8sClient kubernetes.Interface, cnf *rest.Config,
	localCluster liqov1beta1.ClusterID, namespaceManager tenantnamespace.Manager) IdentityReader {
	return NewCertificateIdentityManager(ctx, cl, k8sClient, cnf, localCluster, namespaceManager)
}

// NewCertificateIdentityManager gets a new certificate identity manager.
func NewCertificateIdentityManager(ctx context.Context,
	cl client.Client, k8sClient kubernetes.Interface, cnf *rest.Config,
	localCluster liqov1beta1.ClusterID, namespaceManager tenantnamespace.Manager) IdentityManager {
	idProvider := &certificateIdentityProvider{
		namespaceManager: namespaceManager,
		k8sClient:        k8sClient,
		cl:               cl,
		cnf:              cnf,
	}

	return newIdentityManager(ctx, cl, k8sClient, localCluster, namespaceManager, idProvider)
}

// NewCertificateIdentityProvider gets a new certificate identity approver.
func NewCertificateIdentityProvider(ctx context.Context, cl client.Client, k8sClient kubernetes.Interface,
	cnf *rest.Config,
	localCluster liqov1beta1.ClusterID, namespaceManager tenantnamespace.Manager) IdentityProvider {
	req, err := labels.NewRequirement(remoteTenantCSRLabel, selection.Exists, []string{})
	utilruntime.Must(err)

	csrWatcher := csr.NewWatcher(k8sClient, 0, labels.NewSelector().Add(*req), fields.Everything())
	csrWatcher.Start(ctx)
	idProvider := &certificateIdentityProvider{
		namespaceManager: namespaceManager,
		k8sClient:        k8sClient,
		cl:               cl,
		cnf:              cnf,
		csrWatcher:       csrWatcher,
	}

	return newIdentityManager(ctx, cl, k8sClient, localCluster, namespaceManager, idProvider)
}

// NewIAMIdentityProvider gets a new identity approver to handle IAM identities.
func NewIAMIdentityProvider(ctx context.Context, cl client.Client, k8sClient kubernetes.Interface,
	localCluster liqov1beta1.ClusterID, localAwsConfig *LocalAwsConfig,
	namespaceManager tenantnamespace.Manager) IdentityProvider {
	idProvider := &iamIdentityProvider{
		localAwsConfig: localAwsConfig,
		localClusterID: string(localCluster),
		cl:             cl,
	}

	utilruntime.Must(idProvider.init(ctx))

	return newIdentityManager(ctx, cl, k8sClient, localCluster, namespaceManager, idProvider)
}

func newIdentityManager(ctx context.Context,
	cl client.Client, k8sClient kubernetes.Interface,
	localCluster liqov1beta1.ClusterID,
	namespaceManager tenantnamespace.Manager,
	idProvider IdentityProvider) *identityManager {
	iamTokenManager := &iamTokenManager{
		client:                    k8sClient,
		availableClusterIDSecrets: map[liqov1beta1.ClusterID]types.NamespacedName{},
		tokenFiles:                map[liqov1beta1.ClusterID]string{},
	}
	iamTokenManager.start(ctx)

	return &identityManager{
		client:           cl,
		k8sClient:        k8sClient,
		localCluster:     localCluster,
		namespaceManager: namespaceManager,

		IdentityProvider: idProvider,

		iamTokenManager: iamTokenManager,
	}
}
