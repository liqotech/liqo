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

package forge

import (
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	vk "github.com/liqotech/liqo/pkg/vkMachinery"
)

func testVirtualNode() *offloadingv1beta1.VirtualNode {
	return &offloadingv1beta1.VirtualNode{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vn", Namespace: "tenant-ns"},
		Spec: offloadingv1beta1.VirtualNodeSpec{
			ClusterID:           "remote-cluster",
			KubeconfigSecretRef: &corev1.LocalObjectReference{Name: "kubeconfig-secret"},
		},
	}
}

func testOpts() *offloadingv1beta1.VkOptionsTemplate {
	return &offloadingv1beta1.VkOptionsTemplate{
		Spec: offloadingv1beta1.VkOptionsTemplateSpec{
			CreateNode:          true,
			DisableNetworkCheck: false,
			ContainerImage:      "test-image",
		},
	}
}

func deploymentArgs(t *testing.T, vn *offloadingv1beta1.VirtualNode, opts *offloadingv1beta1.VkOptionsTemplate) []string {
	t.Helper()
	dep := VirtualKubeletDeployment("home-cluster", "liqo-ns", []string{"10.0.0.0/16"}, vn, opts)
	containers := dep.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Fatalf("expected exactly one container, got %d", len(containers))
	}
	return containers[0].Args
}

func expectArg(t *testing.T, args []string, want string) {
	t.Helper()
	if !slices.Contains(args, want) {
		t.Errorf("expected arg %q, got %v", want, args)
	}
}

func expectNoArgWithPrefix(t *testing.T, args []string, prefix string) {
	t.Helper()
	for _, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			t.Errorf("expected no arg with prefix %q, got %q", prefix, arg)
		}
	}
}

func TestVirtualKubeletDeploymentBaseArgs(t *testing.T) {
	args := deploymentArgs(t, testVirtualNode(), testOpts())
	for _, want := range []string{
		"--foreign-cluster-id=remote-cluster",
		"--nodename=test-vn",
		"--tenant-namespace=tenant-ns",
		"--liqo-namespace=liqo-ns",
		"--home-cluster-id=home-cluster",
		"--local-podcidr=10.0.0.0/16",
		"--foreign-kubeconfig-secret-name=kubeconfig-secret",
		"--create-node=true",
		"--node-check-network=true",
	} {
		expectArg(t, args, want)
	}
}

func TestVirtualKubeletDeploymentUpdateStrategy(t *testing.T) {
	dep := VirtualKubeletDeployment("home-cluster", "liqo-ns", []string{"10.0.0.0/16"}, testVirtualNode(), testOpts())

	if dep.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType {
		t.Errorf("expected a RollingUpdate strategy, got %q", dep.Spec.Strategy.Type)
	}
	if dep.Spec.Strategy.RollingUpdate == nil {
		t.Fatal("expected the rolling update strategy to be set")
	}
	if surge := dep.Spec.Strategy.RollingUpdate.MaxSurge; surge == nil || surge.IntVal != 1 {
		t.Errorf("expected maxSurge 1, got %+v", dep.Spec.Strategy.RollingUpdate.MaxSurge)
	}
	if unavailable := dep.Spec.Strategy.RollingUpdate.MaxUnavailable; unavailable == nil || unavailable.IntVal != 0 {
		t.Errorf("expected maxUnavailable 0, got %+v", dep.Spec.Strategy.RollingUpdate.MaxUnavailable)
	}
	if dep.Spec.MinReadySeconds != 0 {
		t.Errorf("expected no minReadySeconds, got %d", dep.Spec.MinReadySeconds)
	}

	probe := dep.Spec.Template.Spec.Containers[0].ReadinessProbe
	if probe == nil || probe.HTTPGet == nil {
		t.Fatal("expected the virtual-kubelet container to have an HTTP readiness probe")
	}
	if probe.HTTPGet.Path != "/readyz" {
		t.Errorf("unexpected readiness probe path: %q", probe.HTTPGet.Path)
	}
	if probe.HTTPGet.Port.IntVal != vk.HealthPort {
		t.Errorf("unexpected readiness probe port: %d", probe.HTTPGet.Port.IntVal)
	}
}

func TestVirtualKubeletDeploymentNoKubeconfigSecretRef(t *testing.T) {
	vn := testVirtualNode()
	vn.Spec.KubeconfigSecretRef = nil
	args := deploymentArgs(t, vn, testOpts())
	expectNoArgWithPrefix(t, args, string(ForeignClusterKubeconfigSecretName))
}

func TestCreateNodeEffectiveValue(t *testing.T) {
	t.Run("defaults to the template value", func(t *testing.T) {
		opts := testOpts()
		opts.Spec.CreateNode = false
		expectArg(t, deploymentArgs(t, testVirtualNode(), opts), "--create-node=false")
	})

	t.Run("the spec value overrides the template", func(t *testing.T) {
		vn := testVirtualNode()
		vn.Spec.CreateNode = ptr.To(true)
		opts := testOpts()
		opts.Spec.CreateNode = false
		expectArg(t, deploymentArgs(t, vn, opts), "--create-node=true")
	})
}

func TestNodeCheckNetworkEffectiveValue(t *testing.T) {
	t.Run("defaults to the template value", func(t *testing.T) {
		opts := testOpts()
		opts.Spec.DisableNetworkCheck = true
		expectArg(t, deploymentArgs(t, testVirtualNode(), opts), "--node-check-network=false")
	})

	t.Run("the spec value overrides the template", func(t *testing.T) {
		vn := testVirtualNode()
		vn.Spec.DisableNetworkCheck = ptr.To(false)
		opts := testOpts()
		opts.Spec.DisableNetworkCheck = true
		expectArg(t, deploymentArgs(t, vn, opts), "--node-check-network=true")
	})
}

func TestEffectiveOffloadingPatch(t *testing.T) {
	t.Run("nil when neither the spec nor the template sets anything", func(t *testing.T) {
		if got := EffectiveOffloadingPatch(testVirtualNode(), testOpts()); got != nil {
			t.Errorf("expected nil effective patch, got %+v", got)
		}
	})

	t.Run("from the template only when the spec patch is nil", func(t *testing.T) {
		opts := testOpts()
		opts.Spec.LabelsNotReflected = []string{"a"}
		opts.Spec.AnnotationsNotReflected = []string{"n1"}
		got := EffectiveOffloadingPatch(testVirtualNode(), opts)
		if !slices.Equal(got.LabelsNotReflected, []string{"a"}) {
			t.Errorf("unexpected labels not reflected: %v", got.LabelsNotReflected)
		}
		if !slices.Equal(got.AnnotationsNotReflected, []string{"n1"}) {
			t.Errorf("unexpected annotations not reflected: %v", got.AnnotationsNotReflected)
		}
	})

	t.Run("from the spec only when the template sets nothing", func(t *testing.T) {
		vn := testVirtualNode()
		vn.Spec.OffloadingPatch = &offloadingv1beta1.OffloadingPatch{
			LabelsNotReflected: []string{"c"},
			NodeSelector:       map[string]string{"pool": "gold"},
		}
		got := EffectiveOffloadingPatch(vn, testOpts())
		if !slices.Equal(got.LabelsNotReflected, []string{"c"}) {
			t.Errorf("unexpected labels not reflected: %v", got.LabelsNotReflected)
		}
		if got.NodeSelector["pool"] != "gold" {
			t.Errorf("the structured fields of the spec patch must be preserved: %+v", got)
		}
	})

	t.Run("merged with the template ones without duplicates, spec first", func(t *testing.T) {
		vn := testVirtualNode()
		vn.Spec.OffloadingPatch = &offloadingv1beta1.OffloadingPatch{
			LabelsNotReflected:      []string{"b", "d"},
			AnnotationsNotReflected: []string{"n2"},
		}
		opts := testOpts()
		opts.Spec.LabelsNotReflected = []string{"a", "b"}
		opts.Spec.AnnotationsNotReflected = []string{"n1", "n2"}
		got := EffectiveOffloadingPatch(vn, opts)
		if want := []string{"b", "d", "a"}; !slices.Equal(got.LabelsNotReflected, want) {
			t.Errorf("unexpected labels not reflected: want %v, got %v", want, got.LabelsNotReflected)
		}
		if want := []string{"n2", "n1"}; !slices.Equal(got.AnnotationsNotReflected, want) {
			t.Errorf("unexpected annotations not reflected: want %v, got %v", want, got.AnnotationsNotReflected)
		}
	})
}
