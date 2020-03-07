package advertisement_operator

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"

	protocolv1 "github.com/netgroup-polito/dronev2/api/advertisement-operator/v1"
	pkg "github.com/netgroup-polito/dronev2/pkg/advertisement-operator"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log logr.Logger
)

// start the broadcaster which sends Advertisement messages
// it reads the ConfigMaps to get the kubeconfigs to the remote clusters and create a client for each of them
// parameters
// - clusterId: the cluster ID of your cluster (must be a UUID)
// - localKubeconfig: the path to the kubeconfig of the local cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - foreignKubeconfig: the path to the kubeconfig of the foreign cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
func StartBroadcaster(clusterId string, localKubeconfig string, foreignKubeconfig string) {
	log = ctrl.Log.WithName("advertisement-broadcaster")
	log.Info("starting broadcaster")

	// get a client to the local cluster
	localClient, err := pkg.NewK8sClient(localKubeconfig, nil)
	if err != nil {
		log.Error(err, "Unable to create client to local cluster")
		return
	}

	// get configMaps containing the kubeconfig of the foreign clusters
	configMaps, err := localClient.CoreV1().ConfigMaps("default").List(metav1.ListOptions{})
	if err != nil {
		log.Error(err, "Unable to list configMaps")
		return
	}
	for _, cm := range configMaps.Items {
		if strings.HasPrefix(cm.Name, "foreign-kubeconfig") {
			go GenerateAdvertisement(localClient, foreignKubeconfig, cm.DeepCopy(), clusterId)
		}
	}
}

// generate an advertisement message every 10 minutes and post it to remote clusters
// parameters
// - localClient: a client to the local kubernetes
// - foreignKubeconfigPath: the path to a kubeconfig file. If set, this file is used to create a client to the foreign cluster. Set it only for debugging purposes
// - cm: the configMap containing the kubeconfig to the foreign cluster. IMPORTANT: the data in the configMap must be named "remote"
func GenerateAdvertisement(localClient *kubernetes.Clientset, foreignKubeconfigPath string, cm *v1.ConfigMap, clusterId string) {
	//TODO: recovering logic if errors occurs

	var remoteClient client.Client
	var err error
	var retry int
	// extract the foreign cluster id from the configMap
	foreignClusterId := cm.Name[len("foreign-kubeconfig-"):]

	// create a CRDclient to the foreign cluster
	for retry = 0; retry < 3; retry++ {
		remoteClient, err = pkg.NewCRDClient(foreignKubeconfigPath, cm)
		if err != nil {
			log.Error(err, "Unable to create client to remote cluster "+foreignClusterId+". Retry in 1 minute")
			time.Sleep(1 * time.Minute)
		} else {
			break
		}
	}
	if retry == 3 {
		log.Error(err, "Failed to create client to remote cluster "+foreignClusterId)
		return
	} else {
		log.Info("created client to remote cluster " + foreignClusterId)
	}

	for {
		nodes, err := localClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "type != virtual-node"})
		if err != nil {
			log.Error(err, "Unable to list nodes")
			return
		}

		adv := CreateAdvertisement(nodes.Items, clusterId)
		err = pkg.CreateOrUpdate(remoteClient, context.Background(), log, adv)
		if err != nil {
			log.Error(err, "Unable to create advertisement on remote cluster "+foreignClusterId)
		} else {
			log.Info("correctly created advertisement on remote cluster " + foreignClusterId)
		}
		time.Sleep(10 * time.Minute)
	}
}

// create advertisement message
func CreateAdvertisement(nodes []v1.Node, clusterId string) protocolv1.Advertisement {

	availability, images := GetClusterResources(nodes)
	prices := ComputePrices(images)

	adv := protocolv1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "advertisement-" + clusterId,
			Namespace: "default",
		},
		Spec: protocolv1.AdvertisementSpec{
			ClusterId:    clusterId,
			Images:       images,
			Availability: availability,
			Prices:       prices,
			Network: protocolv1.NetworkInfo{
				PodCIDR:            GetPodCIDR(nodes),
				GatewayIP:          GetGateway(nodes),
				GatewayPrivateIP:   GetGatewayPrivateIP(),
				SupportedProtocols: nil,
			},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
	return adv
}

func GetPodCIDR(nodes []v1.Node) string {
	//TODO: implement
	token := strings.Split(nodes[0].Spec.PodCIDR, ".")
	podCIDR := token[0] + "." + token[1] + "." + "0" + "." + "0/16"
	return podCIDR
}

func GetGateway(nodes []v1.Node) string {
	//TODO: implement

	return nodes[0].Status.Addresses[0].Address
}

func GetGatewayPrivateIP() string {
	//TODO: implement

	return ""
}

// get cluster resources (cpu, ram and pods) and images
func GetClusterResources(nodes []v1.Node) (v1.ResourceList, []v1.ContainerImage) {
	cpu := resource.Quantity{}
	ram := resource.Quantity{}
	pods := resource.Quantity{}
	images := make([]v1.ContainerImage, 0)

	for _, node := range nodes {
		cpu.Add(*node.Status.Allocatable.Cpu())
		ram.Add(*node.Status.Allocatable.Memory())
		pods.Add(*node.Status.Allocatable.Pods())

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