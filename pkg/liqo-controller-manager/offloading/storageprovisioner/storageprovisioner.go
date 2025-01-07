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

package storageprovisioner

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/sig-storage-lib-external-provisioner/v7/controller"

	"github.com/liqotech/liqo/pkg/consts"
	liqoerrors "github.com/liqotech/liqo/pkg/utils/errors"
)

// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims;persistentvolumes,verbs=get;list;watch;create;delete;update;patch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create

type liqoLocalStorageProvisioner struct {
	client                  client.Client
	virtualStorageClassName string
	storageNamespace        string
	localRealStorageClass   string
}

// NewLiqoLocalStorageProvisioner creates a new liqoLocalStorageProvisioner provisioner.
func NewLiqoLocalStorageProvisioner(ctx context.Context, cl client.Client,
	virtualStorageClassName, storageNamespace, localRealStorageClass string) (controller.Provisioner, error) {
	// ensure that the storage namespace exists
	err := cl.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: storageNamespace,
			Labels: map[string]string{
				consts.StorageNamespaceLabel: "true",
			},
		},
	})
	if liqoerrors.IgnoreAlreadyExists(err) != nil {
		return nil, err
	}

	return &liqoLocalStorageProvisioner{
		client:                  cl,
		virtualStorageClassName: virtualStorageClassName,
		storageNamespace:        storageNamespace,
		localRealStorageClass:   localRealStorageClass,
	}, nil
}
