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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
)

func TestSetNetworkConfigurationArgs(t *testing.T) {
	t.Run("nil configuration removes network args", func(t *testing.T) {
		args := []string{
			"--foo=bar",
			"--remote-pod-cidr=10.0.0.0/16",
			"--remote-pod-cidr-remap=192.168.0.0/16",
		}
		got := SetNetworkConfigurationArgs(args, nil)
		assert.Equal(t, []string{"--foo=bar"}, got)
	})

	t.Run("args without equals sign are preserved", func(t *testing.T) {
		args := []string{
			"--foo",
			"--remote-pod-cidr=10.0.0.0/16",
		}
		got := SetNetworkConfigurationArgs(args, nil)
		assert.Equal(t, []string{"--foo"}, got)
	})

	t.Run("configuration not ready removes network args", func(t *testing.T) {
		cfg := &networkingv1beta1.Configuration{}
		args := []string{"--remote-pod-cidr=10.0.0.0/16"}
		got := SetNetworkConfigurationArgs(args, cfg)
		assert.Equal(t, []string{}, got)
	})

	t.Run("valid configuration appends args", func(t *testing.T) {
		cfg := &networkingv1beta1.Configuration{
			Spec: networkingv1beta1.ConfigurationSpec{
				Remote: networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod:      []networkingv1beta1.CIDR{"10.0.0.0/16", "10.1.0.0/16"},
						External: []networkingv1beta1.CIDR{"172.16.0.0/16"},
					},
				},
			},
			Status: networkingv1beta1.ConfigurationStatus{
				Remote: &networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod:      []networkingv1beta1.CIDR{"192.168.0.0/16", "192.169.0.0/16"},
						External: []networkingv1beta1.CIDR{"10.10.0.0/16"},
					},
				},
				Conditions: []metav1.Condition{
					{Type: networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured, Status: metav1.ConditionTrue},
				},
			},
		}

		got := SetNetworkConfigurationArgs([]string{"--foo=bar"}, cfg)

		assert.Contains(t, got, "--foo=bar")
		assert.Contains(t, got, "--remote-pod-cidr=10.0.0.0/16")
		assert.Contains(t, got, "--remote-pod-cidr=10.1.0.0/16")
		assert.Contains(t, got, "--remote-pod-cidr-remap=192.168.0.0/16")
		assert.Contains(t, got, "--remote-pod-cidr-remap=192.169.0.0/16")
		assert.Contains(t, got, "--remote-external-cidr=172.16.0.0/16")
		assert.Contains(t, got, "--remote-external-cidr-remap=10.10.0.0/16")
	})

	t.Run("updated configuration replaces old args", func(t *testing.T) {
		old := &networkingv1beta1.Configuration{
			Spec: networkingv1beta1.ConfigurationSpec{
				Remote: networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod: []networkingv1beta1.CIDR{"10.0.0.0/16"},
					},
				},
			},
			Status: networkingv1beta1.ConfigurationStatus{
				Remote: &networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod: []networkingv1beta1.CIDR{"192.168.0.0/16"},
					},
				},
				Conditions: []metav1.Condition{
					{Type: networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured, Status: metav1.ConditionTrue},
				},
			},
		}
		args := SetNetworkConfigurationArgs([]string{}, old)

		updated := &networkingv1beta1.Configuration{
			Spec: networkingv1beta1.ConfigurationSpec{
				Remote: networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod: []networkingv1beta1.CIDR{"11.0.0.0/16"},
					},
				},
			},
			Status: networkingv1beta1.ConfigurationStatus{
				Remote: &networkingv1beta1.ClusterConfig{
					CIDR: networkingv1beta1.ClusterConfigCIDR{
						Pod: []networkingv1beta1.CIDR{"193.168.0.0/16"},
					},
				},
				Conditions: []metav1.Condition{
					{Type: networkingv1beta1.ConfigurationConditionNetworkCIDRsConfigured, Status: metav1.ConditionTrue},
				},
			},
		}
		got := SetNetworkConfigurationArgs(args, updated)

		assert.Contains(t, got, "--remote-pod-cidr=11.0.0.0/16")
		assert.Contains(t, got, "--remote-pod-cidr-remap=193.168.0.0/16")
		assert.NotContains(t, got, "--remote-pod-cidr=10.0.0.0/16")
		assert.NotContains(t, got, "--remote-pod-cidr-remap=192.168.0.0/16")
	})
}
