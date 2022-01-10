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

package uninstaller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/pointer"

	"github.com/liqotech/liqo/pkg/discovery"
)

// ScaleDiscoveryDeployment scales the discovery deployment replicas to 0.
func ScaleDiscoveryDeployment(ctx context.Context, client dynamic.Interface, liqoNamespace string) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		deploy, err := getDiscoveryDeployment(ctx, client, liqoNamespace)
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}

		deploy.Spec.Replicas = pointer.Int32(0)
		unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
		if err != nil {
			return err
		}

		_, err = client.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(liqoNamespace).Update(
			ctx, &unstructured.Unstructured{Object: unstr}, metav1.UpdateOptions{},
		)
		return err
	})
}

func getDiscoveryDeployment(ctx context.Context, client dynamic.Interface, liqoNamespace string) (*appsv1.Deployment, error) {
	unstr, err := client.Resource(appsv1.SchemeGroupVersion.WithResource("deployments")).Namespace(liqoNamespace).List(
		ctx, metav1.ListOptions{
			LabelSelector: discovery.DeploymentLabelSelector().String(),
		})
	if err != nil {
		return nil, err
	}

	var deployments appsv1.DeploymentList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr.UnstructuredContent(), &deployments); err != nil {
		return nil, err
	}

	switch len(deployments.Items) {
	case 0:
		err = apierrors.NewNotFound(schema.GroupResource{
			Group:    "apps/v1",
			Resource: "deployments",
		}, "liqo-discovery")
		return nil, err
	case 1:
		return &deployments.Items[0], nil
	default:
		return nil, fmt.Errorf("unexpected number of discovery deployments found: %v", len(deployments.Items))
	}
}
