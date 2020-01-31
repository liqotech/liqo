package controllers

import (
	"context"
	"flag"
	"os"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/advertisement-operator/api/v1beta1"
)

func GenerateAdvertisement() error {

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	freeResources := protocolv1beta1.FreeResource{
		Cpu:      *resource.NewQuantity(int64(runtime.NumCPU()), resource.DecimalSI),
		CpuPrice: *resource.NewQuantity(1, resource.DecimalSI),
		Ram:      *resource.NewQuantity(int64(m.Sys-m.Alloc), resource.BinarySI),
		RamPrice: *resource.NewQuantity(2, resource.DecimalSI),
	}

	adv := protocolv1beta1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adv-sample",
			Namespace: "default",
		},
		Spec: protocolv1beta1.AdvertisementSpec{
			ClusterId:    "cluster1",
			Resources:    getDockerImages(),
			Availability: freeResources,
			Timestamp:    metav1.NewTime(time.Now()),
			Validity:     metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}

	remoteClient, err := newCRDClient("./data/foreignKubeconfig")
	if err != nil {
		return err
	}

	//TODO: update if already exist
	err = remoteClient.Create(context.Background(), &adv, &client.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func getDockerImages() []protocolv1beta1.Resource {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		panic(err)
	}

	dockerImages, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}

	//TODO: logic to decide which images will be in the advertisement and to set the price

	// remove docker images without a name
	for i := 0; i < len(dockerImages); i++ {
		if dockerImages[i].RepoTags == nil {
			dockerImages[i] = dockerImages[len(dockerImages)-1]
			//dockerImages[len(dockerImages)-1] = nil
			dockerImages = dockerImages[:len(dockerImages)-1]
		}
	}

	images := make([]protocolv1beta1.Resource, len(dockerImages))

	for i := 0; i < len(dockerImages); i++ {
		images[i].Image.Names = append(images[i].Image.Names, dockerImages[i].RepoTags[0])
		images[i].Image.SizeBytes = dockerImages[i].Size
		images[i].Price = *resource.NewQuantity(5, resource.DecimalSI)
	}

	return images
}

func newCRDClient(configPath string) (client.Client, error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, errors.Wrap(err, "error building client config")
		}
	} else {
		return nil, err
	}

	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "remote-metrics-addr", ":8081", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "remote-enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             k8sruntime.NewScheme(),
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		Port:               9444,
	})
	if err != nil {
		return nil, err
	}

	_ = clientgoscheme.AddToScheme(mgr.GetScheme())
	_ = protocolv1beta1.AddToScheme(mgr.GetScheme())

	remoteClient := mgr.GetClient()
	return remoteClient, nil
}
