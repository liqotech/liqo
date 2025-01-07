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

package unoffload

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	offloadingv1beta1 "github.com/liqotech/liqo/apis/offloading/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
)

// Options encapsulates the arguments of the offload namespace command.
type Options struct {
	*factory.Factory

	Timeout time.Duration
}

// Run implements the offload namespace command.
func (o *Options) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	s := o.Printer.StartSpinner(fmt.Sprintf("Disabling namespace offloading for %q", o.Namespace))
	nsoff := &offloadingv1beta1.NamespaceOffloading{ObjectMeta: metav1.ObjectMeta{
		Name: consts.DefaultNamespaceOffloadingName, Namespace: o.Namespace}}
	if err := o.CRClient.Delete(ctx, nsoff); client.IgnoreNotFound(err) != nil {
		s.Fail(fmt.Sprintf("Failed disabling namespace offloading: %v", err))
		return err
	}
	s.Success(fmt.Sprintf("Offloading of namespace %q correctly disabled", o.Namespace))

	waiter := wait.NewWaiterFromFactory(o.Factory)
	if err := waiter.ForUnoffloading(ctx, o.Namespace); err != nil {
		return err
	}

	return nil
}
