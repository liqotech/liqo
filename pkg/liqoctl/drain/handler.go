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

package drain

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// Options encapsulates the arguments of the drain command.
type Options struct {
	*factory.Factory

	Name string

	Timeout time.Duration
}

// NewOptions returns a new Options struct.
func NewOptions(f *factory.Factory) *Options {
	return &Options{
		Factory: f,
	}
}

// RunDrainTenant drains a tenant cluster.
func (o *Options) RunDrainTenant(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	tenant, err := getters.GetTenantByName(ctx, o.CRClient, o.Name, corev1.NamespaceAll)
	if err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to get tenant: %v", output.PrettyErr(err)))
		return err
	}

	tenant.Spec.TenantCondition = authv1beta1.TenantConditionDrained
	if err := o.CRClient.Update(ctx, tenant); err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to update tenant: %v", output.PrettyErr(err)))
		return err
	}

	o.Printer.Success.Printfln("Tenant %q drained", o.Name)

	return nil
}
