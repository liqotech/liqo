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

package setup

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	ipamv1alpha1 "github.com/liqotech/liqo/apis/ipam/v1alpha1"
	networkingv1beta1 "github.com/liqotech/liqo/apis/networking/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/test/network/client"
)

const (
	// IPName is the name of the IP resource.
	IPName = "external-ip"
)

// CreateAllIP creates all the IP resources.
func CreateAllIP(ctx context.Context, cl *client.Client) error {
	dstip := "1.1.1.1"
	if err := CreateIP(ctx, cl.Consumer, dstip); err != nil {
		return err
	}
	for k := range cl.Providers {
		if err := CreateIP(ctx, cl.Providers[k], dstip); err != nil {
			return err
		}
	}
	return nil
}

// CreateIP creates an IP resource.
func CreateIP(ctx context.Context, cl ctrlclient.Client, dstip string) error {
	ip := ipamv1alpha1.IP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IPName,
			Namespace: NamespaceName,
		},
		Spec: ipamv1alpha1.IPSpec{
			IP:         networkingv1beta1.IP(dstip),
			Masquerade: ptr.To(true),
		},
	}
	if err := cl.Create(ctx, &ip); err != nil && ctrlclient.IgnoreAlreadyExists(err) != nil {
		return err
	}

	return WaitIPRemapped(ctx, cl)
}

// WaitIPRemapped waits for the IP to be remapped.
func WaitIPRemapped(ctx context.Context, cl ctrlclient.Client) error {
	timeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return wait.PollUntilContextCancel(timeout, time.Second, true, func(ctx context.Context) (done bool, err error) {
		ip := ipamv1alpha1.IP{}
		if err := cl.Get(ctx, ctrlclient.ObjectKey{Name: IPName, Namespace: NamespaceName}, &ip); err != nil {
			return false, err
		}
		if ip.Status.IP == "" {
			return false, nil
		}
		return true, nil
	})
}
