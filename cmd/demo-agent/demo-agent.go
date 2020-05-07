package main

import (
	"context"
	"fmt"
	"github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	agent "github.com/netgroup-polito/dronev2/internal/tray-agent/agent-client"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
)

func main() {
	advClient, err := agent.CreateClient(agent.AcquireConfig())
	if err != nil {
		fmt.Println("client error")
		os.Exit(1)
	}
	var adv v1.Advertisement
	var advList v1.AdvertisementList
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "advertisement-cluster2", Namespace: "default"}}
	err = advClient.Get(context.Background(), req.NamespacedName, &adv)
	if err != nil {
		fmt.Println("error on Get")
		os.Exit(1)
	}
	fmt.Println("adv:")
	fmt.Println(adv)
	err = advClient.List(context.Background(), &advList)
	if err != nil {
		fmt.Println("error on List")
		os.Exit(1)
	}
	for i, ad := range advList.Items {

		str := strings.Builder{}
		prices := ad.Spec.Prices
		CpuPrice := prices["cpu"]
		MemPrice := prices["memory"]
		str.WriteString(fmt.Sprintf("%d-\t%v\n", i+1, ad.Name))
		str.WriteString(fmt.Sprintf("ClusterID: %v\nAvailable Resources:\n", ad.Spec.ClusterId))
		str.WriteString(fmt.Sprintf("\t-cpu = %v [price %v]\n", ad.Spec.Availability.Cpu(), CpuPrice.String()))
		str.WriteString(fmt.Sprintf("\t-memory = %v [price %v]\n", ad.Spec.Availability.Memory(), MemPrice.String()))
		str.WriteString(fmt.Sprintf("\t-pods = %v\n", ad.Spec.Availability.Pods()))
		str.WriteString(fmt.Sprintf("STATUS: %v", ad.Status.AdvertisementStatus))
		advDescr := str.String()
		fmt.Print(advDescr)

	}
}
