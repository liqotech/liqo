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

package utils

import (
	"context"
	"errors"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discovery "k8s.io/client-go/discovery"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	mapsutil "github.com/liqotech/liqo/pkg/utils/maps"
	"github.com/liqotech/liqo/pkg/utils/resource"
)

// EnsureMetrics ensures that the metrics service and service monitor are created or deleted.
func EnsureMetrics(ctx context.Context,
	cl client.Client, s *runtime.Scheme,
	metrics *networkingv1beta1.Metrics, owner metav1.Object) error {
	if metrics == nil || !metrics.Enabled {
		if err := deleteMetricsService(ctx, cl, owner); err != nil {
			klog.Errorf("error while deleting service %q: %v", fmt.Sprintf("%s-metrics", owner.GetName()), err)
			return err
		}

		if err := deleteMetricsServiceMonitor(ctx, cl, owner); err != nil {
			klog.Errorf("error while deleting service monitor %q: %v", fmt.Sprintf("%s-metrics", owner.GetName()), err)
			return err
		}
		return nil
	}

	if metrics.Service != nil {
		if err := createMetricsService(ctx, cl, s, metrics, owner); err != nil {
			klog.Errorf("error while creating service %q: %v", fmt.Sprintf("%s-metrics", owner.GetName()), err)
			return err
		}
	} else {
		if err := deleteMetricsService(ctx, cl, owner); err != nil {
			klog.Errorf("error while deleting service %q: %v", fmt.Sprintf("%s-metrics", owner.GetName()), err)
			return err
		}
	}

	if metrics.ServiceMonitor != nil {
		if err := createMetricsServiceMonitor(ctx, cl, s, metrics, owner); err != nil {
			klog.Errorf("error while creating service monitor %q: %v", fmt.Sprintf("%s-metrics", owner.GetName()), err)
			return err
		}
	} else {
		if err := deleteMetricsServiceMonitor(ctx, cl, owner); err != nil {
			klog.Errorf("error while deleting service monitor %q: %v", fmt.Sprintf("%s-metrics", owner.GetName()), err)
			return err
		}
	}

	return nil
}

// IgnoreAPINotFoundError ignores the error if it is an API not found error (the CRD is not installed).
func IgnoreAPINotFoundError(err error) error {
	var dErr *discovery.ErrGroupDiscoveryFailed
	if errors.As(err, &dErr) {
		return nil
	}
	var noKindErr *meta.NoKindMatchError
	if errors.As(err, &noKindErr) {
		return nil
	}
	return err
}

func deleteMetricsService(ctx context.Context,
	cl client.Client, owner metav1.Object) error {
	return client.IgnoreNotFound(cl.Delete(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", owner.GetName()),
			Namespace: owner.GetNamespace(),
		},
	}))
}

func deleteMetricsServiceMonitor(ctx context.Context,
	cl client.Client, owner metav1.Object) error {
	return IgnoreAPINotFoundError(client.IgnoreNotFound(cl.Delete(ctx, &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", owner.GetName()),
			Namespace: owner.GetNamespace(),
		},
	})))
}

func createMetricsService(ctx context.Context,
	cl client.Client, s *runtime.Scheme,
	metrics *networkingv1beta1.Metrics, owner metav1.Object) error {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", owner.GetName()),
			Namespace: owner.GetNamespace(),
		},
	}
	_, err := resource.CreateOrUpdate(ctx, cl, &svc, func() error {
		// Forge metadata
		mapsutil.SmartMergeLabels(&svc, metrics.Service.Metadata.GetLabels())
		mapsutil.SmartMergeAnnotations(&svc, metrics.Service.Metadata.GetAnnotations())

		// Forge spec
		svc.Spec = metrics.Service.Spec

		// Set owner of the service
		return controllerutil.SetControllerReference(owner, &svc, s)
	})
	return err
}

func createMetricsServiceMonitor(ctx context.Context,
	cl client.Client, s *runtime.Scheme,
	metrics *networkingv1beta1.Metrics, owner metav1.Object) error {
	svcMonitor := monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-metrics", owner.GetName()),
			Namespace: owner.GetNamespace(),
		},
	}
	_, err := resource.CreateOrUpdate(ctx, cl, &svcMonitor, func() error {
		// Forge metadata
		mapsutil.SmartMergeLabels(&svcMonitor, metrics.ServiceMonitor.Metadata.GetLabels())
		mapsutil.SmartMergeAnnotations(&svcMonitor, metrics.ServiceMonitor.Metadata.GetAnnotations())

		// Forge spec
		svcMonitor.Spec = metrics.ServiceMonitor.Spec

		// Set owner of the service
		return controllerutil.SetControllerReference(owner, &svcMonitor, s)
	})
	return err
}
