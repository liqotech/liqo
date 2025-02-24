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

package cordon

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	authv1beta1 "github.com/liqotech/liqo/apis/authentication/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	tenantnamespace "github.com/liqotech/liqo/pkg/tenantNamespace"
	argsutils "github.com/liqotech/liqo/pkg/utils/args"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

// Options encapsulates the arguments of the cordon command.
type Options struct {
	*factory.Factory

	Name      string
	ClusterID argsutils.ClusterIDFlags

	Timeout time.Duration
}

// NewOptions returns a new Options struct.
func NewOptions(f *factory.Factory) *Options {
	return &Options{
		Factory: f,
	}
}

// RunCordonTenant cordons a tenant cluster.
func (o *Options) RunCordonTenant(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	tenant, err := getters.GetTenantByName(ctx, o.CRClient, o.Name, corev1.NamespaceAll)
	if err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to get tenant: %v", output.PrettyErr(err)))
		return err
	}

	if tenant.Spec.TenantCondition != authv1beta1.TenantConditionActive {
		o.Printer.Warning.Printfln("Tenant %q is not active. Activate the tenant to cordon it.", o.Name)
		return nil
	}

	tenant.Spec.TenantCondition = authv1beta1.TenantConditionCordoned
	if err := o.CRClient.Update(ctx, tenant); err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to update tenant: %v", output.PrettyErr(err)))
		return err
	}

	o.Printer.Success.Printfln("Tenant %q cordoned", o.Name)

	return nil
}

// RunCordonResourceSlice cordons a ResourceSlice.
func (o *Options) RunCordonResourceSlice(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	namespaceManager := tenantnamespace.NewManager(o.Factory.KubeClient, o.Factory.CRClient.Scheme())

	ns, err := namespaceManager.GetNamespace(ctx, o.ClusterID.GetClusterID())
	if err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to get tenant namespace: %v", output.PrettyErr(err)))
		return err
	}

	var rs authv1beta1.ResourceSlice
	if err := o.CRClient.Get(ctx, client.ObjectKey{Name: o.Name, Namespace: ns.Name}, &rs); err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to get ResourceSlice: %v", output.PrettyErr(err)))
		return err
	}

	if rs.Annotations == nil {
		rs.Annotations = make(map[string]string)
	}
	rs.Annotations[consts.CordonResourceAnnotation] = "true"

	if err := o.CRClient.Update(ctx, &rs); err != nil {
		o.Printer.CheckErr(fmt.Errorf("unable to update ResourceSlice: %v", output.PrettyErr(err)))
		return err
	}

	o.Printer.Success.Printfln("ResourceSlice %q cordoned", o.Name)

	return nil
}
