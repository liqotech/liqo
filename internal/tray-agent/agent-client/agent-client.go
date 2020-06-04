/*
 Package agent_client provides functions to interact with the kubernetes cluster with Liqo
 running on it
*/
package agent_client

import (
	"context"
	"flag"
	"fmt"
	"github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	advop "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

// AcquireConfig sets the LIQO_PATH and LIQO_KCONFIG env variables.
// LIQO_KCONFIG represents the location of a kubeconfig file in order to let
// the client connect to the local cluster.
//
// The file path - if not expressed with the 'kubeconfig' program argument -
// is set to $HOME/.kube/config .
//
// It returns the address of a string containing the value of $LIQO_KCONFIG
func AcquireConfig() *string {
	//liqoPath := filepath.Join(os.Getenv("HOME"), "liqo")
	liqoPath := "/liqo"
	if err := os.Setenv("LIQO_PATH", liqoPath); err != nil {
		os.Exit(1)
	}
	kubeconfig := flag.String("kubeconfig", filepath.Join(liqoPath, "Konfig.liqo"),
		"[OPT] absolute path to the kubeconfig file."+
			" Default = /liqo/Konfig.liqo")
	flag.Parse()
	if err := os.Setenv("LIQO_KCONFIG", *kubeconfig); err != nil {
		os.Exit(1)
	}
	return kubeconfig
}

// CreateClient creates a client to a k8s cluster, using a kubeconfig file whose location is passed as parameter
// or retrieved from $LIQO_KCONFIG
func CreateClient(kubeconfigPath *string) (client.Client, error) {
	if kubeconfigPath != nil {
		return advop.NewCRDClient(*kubeconfigPath, nil)
	} else {
		path, pres := os.LookupEnv("LIQO_KCONFIG")
		if pres {
			return advop.NewCRDClient(path, nil)
		} else {
			return advop.NewCRDClient("", nil)
		}
	}
}

// ListAdvertisements returns a slice containing the human-readable description
// of each Advertisement currently inside the cluster associated to the client
func ListAdvertisements(c *client.Client) (advDescriptionList []string, err error) {
	advClient := *c
	var advList v1.AdvertisementList
	err = advClient.List(context.Background(), &advList)
	if err != nil {
		return nil, err
	}
	for i, adv := range advList.Items {

		str := strings.Builder{}
		str.WriteString(fmt.Sprintf("❨%02d❩ ⟹\t%s", i+1, DescribeAdvertisement(&adv)))
		advDescriptionList = append(advDescriptionList, str.String())
	}
	return advDescriptionList, nil
}

func DescribeAdvertisement(adv *v1.Advertisement) string {
	str := strings.Builder{}
	prices := adv.Spec.Prices
	CpuPrice := prices["cpu"]
	MemPrice := prices["memory"]
	str.WriteString(fmt.Sprintf("%v\n", adv.Name))
	str.WriteString(fmt.Sprintf("\t• ClusterID: %v\n\t• Available Resources:\n", adv.Spec.ClusterId))
	str.WriteString(fmt.Sprintf("\t\t-cpu = %v [price %v]\n", adv.Spec.ResourceQuota.Hard.Cpu(), CpuPrice.String()))
	str.WriteString(fmt.Sprintf("\t\t-memory = %v [price %v]\n", adv.Spec.ResourceQuota.Hard.Memory(), MemPrice.String()))
	str.WriteString(fmt.Sprintf("\t\t-pods = %v\n", adv.Spec.ResourceQuota.Hard.Pods()))
	str.WriteString(fmt.Sprintf("\t• STATUS: %v", adv.Status.AdvertisementStatus))
	return str.String()
}
