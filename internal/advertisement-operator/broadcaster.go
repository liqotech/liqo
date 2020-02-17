package advertisement_operator

import (
	"context"
	"runtime"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockerclient "github.com/docker/docker/client"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	protocolv1beta1 "github.com/netgroup-polito/dronev2/api/v1beta1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"
)

// generate an advertisement message every 10 minutes and post it to remote clusters
// parameters
// - localClient: a client to the local kubernetes
// - foreignKubeconfigPath: the path to a kubeconfig file. If set, this file is used to create a client to the foreign cluster
// - configMapName: the name of the configMap containing the kubeconfig to the foreign cluster. If foreignKubeconfigPath is set it is ignored
//					IMPORTANT: the data in the configMap must be named "remote"
func GenerateAdvertisement(localClient client.Client, foreignKubeconfigPath string, configMapName string) {
	//TODO: recovering logic if errors occurs

	// give time to the cache to be started
	time.Sleep(5*time.Second)

	log := ctrl.Log.WithName("advertisement-broadcaster")
	log.Info("starting broadcaster")
	remoteClient, err := pkg.NewCRDClient(foreignKubeconfigPath, configMapName, localClient)
	if err != nil {
		log.Error(err, "Unable to create client to remote cluster")
	}
	log.Info("created client to remote cluster" )

	for {
		var nodes v1.NodeList
		err := localClient.List(context.Background(), &nodes)
		if err != nil {
			log.Error(err, "Unable to list nodes")
		}
		//TODO: filter nodes (e.g. prune all virtual-kubelet)

		adv := CreateAdvertisement(nodes.Items)
		err = pkg.CreateOrUpdate(remoteClient, context.Background(), log, adv)
		if err != nil {
			log.Error(err, "Unable to create advertisement on remote cluster")
		}
		log.Info("correctly created advertisement on remote cluster" )
		time.Sleep(10 * time.Minute)
	}
}

// create advertisement message
func CreateAdvertisement(nodes []v1.Node) protocolv1beta1.Advertisement {

	availability, images := GetClusterResources(nodes)
	prices := ComputePrices(images)

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
func GetClusterResources(nodes []v1.Node) (v1.ResourceList, []v1.ContainerImage) {
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
func ComputePrices(images []v1.ContainerImage) v1.ResourceList {
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
	prices := ComputePrices(images)
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
