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

package unpeer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	liqov1beta1 "github.com/liqotech/liqo/apis/core/v1beta1"
	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/liqoctl/output"
	"github.com/liqotech/liqo/pkg/liqoctl/unauthenticate"
	"github.com/liqotech/liqo/pkg/liqoctl/wait"
	"github.com/liqotech/liqo/pkg/utils/getters"
)

func deleteResourceSlicesByClusterID(ctx context.Context, f *factory.Factory,
	remoteClusterID liqov1beta1.ClusterID, waitForActualDeletion bool) error {
	s := f.Printer.StartSpinner("Deleting resourceslices")

	rsSelector := labels.Set{
		consts.ReplicationRequestedLabel:   consts.ReplicationRequestedLabelValue,
		consts.ReplicationDestinationLabel: string(remoteClusterID),
	}

	resourceSlices, err := getters.ListResourceSlicesByLabel(ctx, f.CRClient,
		corev1.NamespaceAll, labels.SelectorFromSet(rsSelector))

	switch {
	case err != nil:
		s.Fail("Error while retrieving resourceslices: ", output.PrettyErr(err))
		return err
	case len(resourceSlices) == 0:
		s.Success("No resourceslices found")
	default:
		for i := range resourceSlices {
			if err := client.IgnoreNotFound(f.CRClient.Delete(ctx, &resourceSlices[i])); err != nil {
				s.Fail("Error while deleting resourceslice ", client.ObjectKeyFromObject(&resourceSlices[i]), ": ", output.PrettyErr(err))
				return err
			}
		}
		s.Success("All resourceslices deleted")

		if waitForActualDeletion {
			// wait for all resourceslices to be deleted
			waiter := wait.NewWaiterFromFactory(f)
			if err := waiter.ForResourceSlicesAbsence(ctx, corev1.NamespaceAll, labels.SelectorFromSet(rsSelector)); err != nil {
				return err
			}
		}
	}

	return nil
}

func deleteVirtualNodesByClusterID(ctx context.Context, f *factory.Factory,
	remoteClusterID liqov1beta1.ClusterID, waitForActualDeletion bool) error {
	s := f.Printer.StartSpinner("Deleting virtualnodes")

	virtualNodes, err := getters.ListVirtualNodesByClusterID(ctx, f.CRClient, remoteClusterID)

	switch {
	case err != nil:
		s.Fail("Error while retrieving virtualnodes: ", output.PrettyErr(err))
		return err
	case len(virtualNodes) == 0:
		s.Success("No virtualnodes found")
	default:
		for i := range virtualNodes {
			if err := client.IgnoreNotFound(f.CRClient.Delete(ctx, &virtualNodes[i])); err != nil {
				s.Fail("Error while deleting virtualnode ", client.ObjectKeyFromObject(&virtualNodes[i]), ": ", output.PrettyErr(err))
				return err
			}
		}
		s.Success("All virtualnodes deleted")

		if waitForActualDeletion {
			// wait for all virtualnodes to be deleted
			waiter := wait.NewWaiterFromFactory(f)
			if err := waiter.ForVirtualNodesAbsence(ctx, remoteClusterID); err != nil {
				return err
			}
		}
	}

	return nil
}

// func (o *Options) unpeerConsumerClusterOnly(ctx context.Context) error {

// 	fmt.Print("Sono entrata nella funzione\n")
// 	// Disabilita offloading (ResourceSlices + VirtualNodes)
// 	if err := o.disableOffloading(ctx); err != nil {
// 		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable offloading: %w", err))
// 		return err
// 	}

// 	fmt.Print("Sono dopo la disable offloading\n")
// 	// Disabilita networking (solo lato consumer)
// 	if err := o.disableNetworking(ctx); err != nil {
// 		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable networking: %w", err))
// 		return err
// 	}

// 	fmt.Print("Sono dopo la disable networking\n")
// 	// Rimuove il ForeignCluster locale associato al provider
// 	fcList := &liqov1beta1.ForeignClusterList{}
// 	if err := o.LocalFactory.CRClient.List(ctx, fcList); err != nil {
// 		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to get foreignCluster: %w", err))
// 		return err
// 	}

// 	fmt.Println("lista di oggetti\n", fcList)
// 	found := false
// 	for i := range fcList.Items {
// 		// fmt.Println("foreigncluster ------", fcList.Items[i].Spec.ClusterID)
// 		fmt.Println("clusterid ------", string(fcList.Items[i].Spec.ClusterID))
// 		if string(fcList.Items[i].Spec.ClusterID) == "milan" {
// 			fmt.Print("Dentro l'f del FOR\n")
// 			fc := &fcList.Items[i]
// 			// fmt.Println("foreigncluster ------", fc)
// 			o.LocalFactory.Printer.Verbosef("Eliminazione del ForeignCluster locale %q...\n", fc.Name)
// 			if err := o.LocalFactory.CRClient.Delete(ctx, fc); err != nil {
// 				// fmt.Errorf("errore nella cancellazione del ForeignCluster %q: %w", fc.Name, err)
// 				o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to delete foreignCluster%q: %w", fc.Name, err))
// 				return err
// 			}
// 			found = true
// 			break
// 		}
// 	}
// 	if !found {
// 		fmt.Print("Dentro la not found\n")
// 		o.LocalFactory.Printer.Verbosef(" Nessun ForeignCluster con ID %q trovato nel cluster locale.\n", o.providerClusterID)
// 	}

// 	fmt.Println("Valore del deletenamespace", o.DeleteNamespace)

// 	// Elimina il tenant namespace se richiesto
// 	o.DeleteNamespace = true
// 	if o.DeleteNamespace {

// 		consumer := unauthenticate.NewCluster(o.LocalFactory)
// 		if err := consumer.DeleteTenantNamespace(ctx, o.providerClusterID, o.Wait); err != nil {
// 			// o.LocalFactory.Printer.Warningf("⚠️  Errore nella cancellazione del tenant namespace: %v\n", err)
// 			o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to delete tenant namespace: %v", err))
// 			return err
// 		}
// 	}
// 	fmt.Print("PRIMA DEL RETURN \n")
// 	o.LocalFactory.Printer.Verbosef("Unpeering lato consumer completato con successo.")
// 	return nil
// }

// L'OBIETTIVO è DI ELIMINARE QUESTA FUNZIONE E DI INSERIRE SOLAMENTE LA CONDIZIONE DI IF NEL CODICE PRINCIPALE
func (o *Options) unpeerConsumerClusterOnly(ctx context.Context) error {

	fmt.Print("Sono entrata nella funzione\n")
	// Disabilita offloading (ResourceSlices + VirtualNodes)
	if err := o.disableOffloading(ctx); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable offloading: %w", err))
		return err
	}
	fmt.Print("Sono dopo la disable offloading\n")

	// Disable authentication
	if err := o.disableAuthentication(ctx); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable authentication: %w", err))
		return err
	}
	fmt.Print("Sono dopo la disable autentication\n")

	// Disabilita networking (solo lato consumer)
	if err := o.disableNetworking(ctx); err != nil {
		o.LocalFactory.Printer.CheckErr(fmt.Errorf("unable to disable networking: %w", err))
		return err
	}
	fmt.Print("Sono dopo la disable networking\n")

	consumer := unauthenticate.NewCluster(o.LocalFactory)

	if err := consumer.DeleteTenantNamespace(ctx, o.providerClusterID, o.Wait); err != nil {
		return err
	}

	return nil
}
