package advertisement_operator

import (
	"context"
	"github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
	"os"
	"runtime"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/api/v1beta1"
)

// generate an advertisement message every 10 minutes and post it to remote clusters
func GenerateAdvertisement(client client.Client) {
	//TODO: recovering logic if errors occurs

	log := ctrl.Log.WithName("advertisement-broadcaster")

	remoteClient, err := newCRDClient("./data/foreignKubeconfig")
	if err != nil {
		log.Error(err, "Unable to create client to remote cluster")
	}

	for {
		adv := createAdvertisement(client)
		err = advertisement_operator.CreateOrUpdate(remoteClient, context.Background(), log, adv)
		if err != nil {
			log.Error(err, "Unable to create advertisement on remote cluster")
		}
		time.Sleep(10 * time.Minute)
	}
}

// create advertisement message
func createAdvertisement(client client.Client) protocolv1beta1.Advertisement {

	var nodes v1.NodeList
	err := client.List(context.Background(), &nodes)
	if err != nil {
		//TODO
	}

	//TODO: filter nodes (e.g. prune all virtual-kubelet)

	availability, images := getClusterResources(nodes.Items)
	prices := computePrices(images)

	adv := protocolv1beta1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adv-sample",
			Namespace: "default",
		},
		Spec: protocolv1beta1.AdvertisementSpec{
			ClusterId:    "cluster1",
			Images:       images,
			Availability: availability,
			Prices:       prices,
			Timestamp:    metav1.NewTime(time.Now()),
			Validity:     metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
	return adv
}

// get cluster resources (cpu, ram and pods) and images
func getClusterResources(nodes []v1.Node) (v1.ResourceList, []v1.ContainerImage) {
	cpu := resource.Quantity{}
	ram := resource.Quantity{}
	pods := resource.Quantity{}
	images := make([]v1.ContainerImage, 0)

	for _, node := range nodes {
		cpu.Add(*node.Status.Capacity.Cpu())
		ram.Add(*node.Status.Capacity.Memory())
		pods.Add(*node.Status.Capacity.Pods())

		//TODO: filter images
		for _, image := range node.Status.Images {
			images = append(images, image)
		}
	}
	availability := v1.ResourceList{}
	availability[v1.ResourceCPU] = cpu
	availability[v1.ResourceMemory] = ram
	availability[v1.ResourcePods] = pods
	return availability, images
}

// create prices resource for advertisement
func computePrices(images []v1.ContainerImage) v1.ResourceList {
	//TODO: logic to set prices
	prices := v1.ResourceList{}
	prices[v1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)
	prices[v1.ResourceMemory] = *resource.NewQuantity(2, resource.DecimalSI)
	for _, image := range images {
		for _, name := range image.Names {
			prices[v1.ResourceName(name)] = *resource.NewQuantity(5, resource.DecimalSI)
		}
	}
	return prices
}

// create advertisement with all system resources
func createAdvertisementWithAllSystemResources() protocolv1beta1.Advertisement {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	freeResources := v1.ResourceList{}

	freeResources[v1.ResourceCPU] = *resource.NewQuantity(int64(runtime.NumCPU()), resource.DecimalSI)
	freeResources[v1.ResourceMemory] = *resource.NewQuantity(int64(m.Sys-m.Alloc), resource.BinarySI)
	images := getDockerImages()
	prices := computePrices(images)
	adv := protocolv1beta1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "adv-sample",
			Namespace: "default",
		},
		Spec: protocolv1beta1.AdvertisementSpec{
			ClusterId:    "cluster1",
			Images:       images,
			Availability: freeResources,
			Prices:       prices,
			Timestamp:    metav1.NewTime(time.Now()),
			Validity:     metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
	return adv
}

// get all local docker images
func getDockerImages() []v1.ContainerImage {
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		panic(err)
	}

	dockerImages, err := cli.ImageList(context.Background(), dockertypes.ImageListOptions{})
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

	images := make([]v1.ContainerImage, len(dockerImages))

	for i := 0; i < len(dockerImages); i++ {
		images[i].Names = append(images[i].Names, dockerImages[i].RepoTags[0])
		images[i].SizeBytes = dockerImages[i].Size
	}

	return images
}

// create a client to a cluster given its kubeconfig
func newCRDClient(configPath string) (client.Client, error) {
	var config *rest.Config

	// Check if the kubeConfig file exists.
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		// Get the kubeconfig from the filepath.
		config, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	scheme := k8sruntime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = protocolv1beta1.AddToScheme(scheme)

	remoteClient, err := client.New(config, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return remoteClient, nil
}
