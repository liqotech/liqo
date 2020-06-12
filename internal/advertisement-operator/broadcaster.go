package advertisement_operator

import (
	"context"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"github.com/liqoTech/liqo/internal/discovery/kubeconfig"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log logr.Logger
)

type AdvertisementBroadcaster struct {
	//TODO: transform these functions in methods to avoid all that parameters
}

// start the broadcaster which sends Advertisement messages
// it reads the Secret to get the kubeconfig to the remote cluster and create a client for it
// parameters
// - clusterId: the cluster ID of your cluster (must be a UUID)
// - localKubeconfig: the path to the kubeconfig of the local cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - foreignKubeconfig: the path to the kubeconfig of the foreign cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - gatewayIP: the IP address of the gateway node
// - gatewayPrivateIP: the private IP address of the gateway node
// - peeringRequestName: the name of the PeeringRequest containing the reference to the secret with the kubeconfig for creating Advertisements CR on foreign cluster
func StartBroadcaster(clusterId string, localKubeconfig string, foreignKubeconfig string, gatewayIP string, gatewayPrivateIP string, peeringRequestName string) {
	log = ctrl.Log.WithName("advertisement-broadcaster")
	log.Info("starting broadcaster")

	// get a client to the local cluster
	localClient, err := pkg.NewK8sClient(localKubeconfig, nil, nil)
	if err != nil {
		log.Error(err, "Unable to create client to local cluster")
		return
	}
	// TODO: maybe we can use only the CRD client
	localCRDClient, err := pkg.NewCRDClient(localKubeconfig, nil, nil)
	if err != nil {
		log.Error(err, "Unable to create CRD client to local cluster")
		return
	}

	// get configuration from PeeringRequest CR
	discoveryClient, err := clients.NewDiscoveryClient()
	if err != nil {
		log.Error(err, "Unable to create a discovery client for local cluster")
		return
	}

	pr, err := discoveryClient.PeeringRequests().Get(peeringRequestName, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get PeeringRequest "+peeringRequestName)
		return
	}

	secretForAdvertisementCreation, err := localClient.CoreV1().Secrets(pr.Spec.KubeConfigRef.Namespace).Get(pr.Spec.KubeConfigRef.Name, metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get PeeringRequest secret")
	}

	//TODO: get the name of the service account
	sa, err := localClient.CoreV1().ServiceAccounts(pr.Spec.Namespace).Get("broadcaster", metav1.GetOptions{})
	if err != nil {
		log.Error(err, "Unable to get ServiceAccount broadcaster")
	}
	GenerateAdvertisement(localClient, localCRDClient, foreignKubeconfig, secretForAdvertisementCreation, sa, clusterId, pr.Name, gatewayIP, gatewayPrivateIP)
}

// generate an advertisement message every 10 minutes and post it to remote clusters
// parameters
// - localClient: a client to the local kubernetes
// - localCRDClient: a CRD client to the local kubernetes
// - foreignKubeconfigPath: the path to a kubeconfig file. If set, this file is used to create a client to the foreign cluster. Set it only for debugging purposes
// - secret: the Secret containing the kubeconfig to create Advertisement on the foreign cluster
// - sa: the serviceAccount related to the permissions we want to give to the foreign cluster
func GenerateAdvertisement(localClient *kubernetes.Clientset, localCRDClient client.Client, foreignKubeconfigPath string,
	secret *corev1.Secret, sa *corev1.ServiceAccount,
	localClusterId, foreignClusterId, gatewayIP, gatewayPrivateIP string) {
	//TODO: recovering logic if errors occurs

	var remoteClient client.Client
	var err error
	var retry int
	var once sync.Once

	// create a CRDclient to the foreign cluster
	for retry = 0; retry < 3; retry++ {
		remoteClient, err = pkg.NewCRDClient(foreignKubeconfigPath, nil, secret)
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
		// get physical and virtual nodes in the cluster
		physicalNodes, err := localClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "type != virtual-node"})
		if err != nil {
			log.Error(err, "Could not get physical nodes, retry in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		virtualNodes, err := localClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "type = virtual-node"})
		if err != nil {
			log.Error(err, "Could not get virtual nodes, retry in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		// get resources used by pods in the cluster
		fieldSelector, err := fields.ParseSelector("status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
		if err != nil {
			log.Error(err, "Could not parse field selector")
			continue
		}
		nodeNonTerminatedPodsList, err := localClient.CoreV1().Pods("").List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
		if err != nil {
			log.Error(err, "Could not list pods, retry in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		reqs, limits := GetAllPodsResources(nodeNonTerminatedPodsList)
		// compute resources to be announced to the other cluster
		availability, images := ComputeAnnouncedResources(physicalNodes, reqs)

		// create the kubeconfig to allow the foreign cluster to create resources on local cluster
		kubeconfigForForeignCluster, err := kubeconfig.CreateKubeConfig(sa.Name, sa.Namespace)
		if err != nil {
			log.Error(err, "Unable to create Kubeconfig")
			continue
		}
		// put the kubeconfig in a Secret, which is created on the foreign cluster
		kubeconfigSecret := corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vk-kubeconfig-secret-" + localClusterId,
				Namespace: sa.Namespace,
			},
			Data: nil,
			StringData: map[string]string{
				"kubeconfig": kubeconfigForForeignCluster,
			},
		}

		err = pkg.CreateOrUpdate(remoteClient, context.Background(), log, kubeconfigSecret)
		if err != nil {
			// secret not created, without it the vk cannot be launched: just log and try again
			log.Error(err, "Unable to create secret on remote cluster "+foreignClusterId)
		} else {
			// secret correctly created on foreign cluster, now create the Advertisement to trigger the creation of the virtual-kubelet
			adv := CreateAdvertisement(localClusterId, gatewayIP, gatewayPrivateIP, physicalNodes, virtualNodes, availability, images, limits, kubeconfigSecret)
			err = pkg.CreateOrUpdate(remoteClient, context.Background(), log, &adv)
			if err != nil {
				log.Error(err, "Unable to create advertisement on remote cluster "+foreignClusterId)
			} else {
				// Advertisement created, set the owner reference of the secret so that it is deleted when the adv is removed
				log.Info("correctly created advertisement on remote cluster " + foreignClusterId)
				adv.Kind = "Advertisement"
				adv.APIVersion = protocolv1.GroupVersion.String()
				kubeconfigSecret.SetOwnerReferences(pkg.GetOwnerReference(adv))
				err = remoteClient.Update(context.Background(), &kubeconfigSecret)
				if err != nil {
					log.Error(err, "Unable to update secret "+kubeconfigSecret.Name)
				}
				// start the remote watcher over this Advertisement; the watcher must be launched only once
				go once.Do(func() {
					scheme := runtime.NewScheme()
					_ = clientgoscheme.AddToScheme(scheme)
					_ = protocolv1.AddToScheme(scheme)
					WatchAdvertisement(localCRDClient, scheme, foreignKubeconfigPath, secret, localClusterId, foreignClusterId)
				})
			}
		}
		time.Sleep(10 * time.Minute)
	}
}

// create advertisement message
func CreateAdvertisement(clusterId string, gatewayIP string, gatewayPrivateIp string,
	physicalNodes *corev1.NodeList, virtualNodes *corev1.NodeList,
	availability corev1.ResourceList, images []corev1.ContainerImage, limits corev1.ResourceList, secret corev1.Secret) protocolv1.Advertisement {

	// set prices field
	prices := ComputePrices(images)
	// use virtual nodes to build neighbours
	neighbours := make(map[corev1.ResourceName]corev1.ResourceList)
	for _, vnode := range virtualNodes.Items {
		neighbours[corev1.ResourceName(strings.TrimPrefix(vnode.Name, "vk-"))] = vnode.Status.Allocatable
	}

	adv := protocolv1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "advertisement-" + clusterId,
			Namespace: "default",
		},
		Spec: protocolv1.AdvertisementSpec{
			ClusterId: clusterId,
			Images:    images,
			LimitRange: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{
					{
						Type:                 "",
						Max:                  limits,
						Min:                  nil,
						Default:              nil,
						DefaultRequest:       nil,
						MaxLimitRequestRatio: nil,
					},
				},
			},
			ResourceQuota: corev1.ResourceQuotaSpec{
				Hard:          availability,
				Scopes:        nil,
				ScopeSelector: nil,
			},
			Neighbors:  neighbours,
			Properties: nil,
			Prices:     prices,
			Network: protocolv1.NetworkInfo{
				PodCIDR:            GetPodCIDR(physicalNodes.Items),
				GatewayIP:          gatewayIP,
				GatewayPrivateIP:   gatewayPrivateIp,
				SupportedProtocols: nil,
			},
			KubeConfigRef: corev1.ObjectReference{
				Kind:       secret.Kind,
				Namespace:  secret.Namespace,
				Name:       secret.Name,
				UID:        secret.UID,
				APIVersion: secret.APIVersion,
			},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
	return adv
}

func GetPodCIDR(nodes []corev1.Node) string {
	var podCIDR string
	token := strings.Split(nodes[0].Spec.PodCIDR, ".")
	if len(token) >= 2 {
		podCIDR = token[0] + "." + token[1] + "." + "0" + "." + "0/16"
	} else {
		podCIDR = "172.17.0.0/16"
	}
	return podCIDR
}

func GetGateway(nodes []corev1.Node) string {
	return nodes[0].Status.Addresses[0].Address
}

func GetGatewayPrivateIP() string {
	//TODO: implement

	return ""
}

// get resources used by pods on physical nodes
func GetAllPodsResources(nodeNonTerminatedPodsList *corev1.PodList) (requests corev1.ResourceList, limits corev1.ResourceList) {
	// remove pods on virtual nodes
	for i, pod := range nodeNonTerminatedPodsList.Items {
		if strings.HasPrefix(pod.Spec.NodeName, "vk-") {
			nodeNonTerminatedPodsList.Items[i] = corev1.Pod{}
		}
	}
	requests, limits = getPodsTotalRequestsAndLimits(nodeNonTerminatedPodsList)
	return requests, limits
}

func getPodsTotalRequestsAndLimits(podList *corev1.PodList) (reqs map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity) {
	reqs, limits = map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, pod := range podList.Items {
		podReqs, podLimits := resourcehelper.PodRequestsAndLimits(&pod)
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = podReqValue.DeepCopy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = podLimitValue.DeepCopy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	return
}

// get cluster resources (cpu, ram and pods) and images
func GetClusterResources(nodes []corev1.Node) (corev1.ResourceList, []corev1.ContainerImage) {
	cpu := resource.Quantity{}
	ram := resource.Quantity{}
	pods := resource.Quantity{}
	clusterImages := make([]corev1.ContainerImage, 0)

	for _, node := range nodes {
		cpu.Add(*node.Status.Allocatable.Cpu())
		ram.Add(*node.Status.Allocatable.Memory())
		pods.Add(*node.Status.Allocatable.Pods())

		nodeImages := GetNodeImages(node)
		clusterImages = append(clusterImages, nodeImages...)
	}
	availability := corev1.ResourceList{}
	availability[corev1.ResourceCPU] = cpu
	availability[corev1.ResourceMemory] = ram
	availability[corev1.ResourcePods] = pods
	return availability, clusterImages
}

func GetNodeImages(node corev1.Node) []corev1.ContainerImage {
	images := make([]corev1.ContainerImage, 0)

	for _, image := range node.Status.Images {
		for _, name := range image.Names {
			// TODO: policy to decide which images to announce
			if !strings.Contains(name, "k8s") {
				images = append(images, image)
			}
		}
	}

	return images
}

// create announced resources for advertisement
func ComputeAnnouncedResources(physicalNodes *corev1.NodeList, reqs corev1.ResourceList) (availability corev1.ResourceList, images []corev1.ContainerImage) {
	// get allocatable resources in all the physical nodes
	allocatable, images := GetClusterResources(physicalNodes.Items)

	// subtract used resources from available ones to have available resources
	cpu := allocatable.Cpu().DeepCopy()
	cpu.Sub(reqs.Cpu().DeepCopy())
	mem := allocatable.Memory().DeepCopy()
	mem.Sub(reqs.Memory().DeepCopy())
	pods := allocatable.Pods().DeepCopy()

	// TODO: policy to decide how many resources to announce
	cpu.Set(cpu.Value() / 2)
	mem.Set(mem.Value() / 2)
	pods.Set(pods.Value() / 2)
	availability = corev1.ResourceList{
		corev1.ResourceCPU:    cpu,
		corev1.ResourceMemory: mem,
		corev1.ResourcePods:   pods,
	}

	return availability, images
}

// create prices resource for advertisement
func ComputePrices(images []corev1.ContainerImage) corev1.ResourceList {
	//TODO: logic to set prices
	prices := corev1.ResourceList{}
	prices[corev1.ResourceCPU] = *resource.NewQuantity(1, resource.DecimalSI)
	prices[corev1.ResourceMemory] = resource.MustParse("2m")
	for _, image := range images {
		for _, name := range image.Names {
			prices[corev1.ResourceName(name)] = *resource.NewQuantity(5, resource.DecimalSI)
		}
	}
	return prices
}
