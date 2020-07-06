package advertisement_operator

import (
	"context"
	"errors"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery/kubeconfig"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog"
	"strings"
	"sync"
	"time"

	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AdvertisementBroadcaster struct {
	// local-related variables
	LocalClient     *kubernetes.Clientset
	LocalCRDClient  client.Client
	DiscoveryClient *v1alpha1.CRDClient
	// remote-related variables
	KubeconfigSecretForForeign *corev1.Secret // secret containing the kubeconfig that will be sent to the foreign cluster
	RemoteClient               client.Client  // client to create Advertisements and Secrets on the foreign cluster
	// configuration variables
	HomeClusterId    string
	ForeignClusterId string
	GatewayIP        string
	GatewayPrivateIP string
	ClusterConfig    policyv1.AdvertisementConfig
}

// start the broadcaster which sends Advertisement messages
// it reads the Secret to get the kubeconfig to the remote cluster and create a client for it
// parameters
// - homeClusterId: the cluster ID of your cluster (must be a UUID)
// - localKubeconfigPath: the path to the kubeconfig of the local cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - foreignKubeconfigPath: the path to the kubeconfig of the foreign cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - gatewayIP: the IP address of the gateway node
// - gatewayPrivateIP: the private IP address of the gateway node
// - peeringRequestName: the name of the PeeringRequest containing the reference to the secret with the kubeconfig for creating Advertisements CR on foreign cluster
// - saName: The name of the ServiceAccount used to create the kubeconfig that will be sent to the foreign cluster with the permissions to create resources on local cluster
func StartBroadcaster(homeClusterId, localKubeconfigPath, foreignKubeconfigPath, gatewayIP, gatewayPrivateIP, peeringRequestName, saName string) error {
	klog.V(6).Info("starting broadcaster")

	// get a client to the local cluster
	localClient, err := pkg.NewK8sClient(localKubeconfigPath, nil, nil)
	if err != nil {
		klog.Errorln(err, "Unable to create client to local cluster")
		return err
	}
	// TODO: maybe we can use only the CRD client
	localCRDClient, err := pkg.NewCRDClient(localKubeconfigPath, nil, nil)
	if err != nil {
		klog.Errorln(err, "Unable to create CRD client to local cluster")
		return err
	}

	config, err := v1alpha1.NewKubeconfig(localKubeconfigPath, &discoveryv1.GroupVersion)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	discoveryClient, err := v1alpha1.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}

	tmp, err := discoveryClient.Resource("peeringrequests").Get(peeringRequestName, metav1.GetOptions{})
	if err != nil {
		klog.Errorln(err, "Unable to get PeeringRequest "+peeringRequestName)
		return err
	}
	pr, ok := tmp.(*discoveryv1.PeeringRequest)
	if !ok {
		klog.Errorln(err, "retrieved object is not a PeeringRequest")
		return errors.New("retrieved object is not a PeeringRequest")
	}

	foreignClusterId := pr.Name

	secretForAdvertisementCreation, err := localClient.CoreV1().Secrets(pr.Spec.KubeConfigRef.Namespace).Get(pr.Spec.KubeConfigRef.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorln(err, "Unable to get PeeringRequest secret")
		return err
	}

	var remoteClient client.Client
	var retry int

	// create a CRD-client to the foreign cluster
	for retry = 0; retry < 3; retry++ {
		remoteClient, err = pkg.NewCRDClient(foreignKubeconfigPath, nil, secretForAdvertisementCreation)
		if err != nil {
			klog.Errorln(err, "Unable to create client to remote cluster "+foreignClusterId+". Retry in 1 minute")
			time.Sleep(1 * time.Minute)
		} else {
			break
		}
	}
	if retry == 3 {
		klog.Errorln(err, "Failed to create client to remote cluster "+foreignClusterId)
		return err
	} else {
		klog.Info("Correctly created client to remote cluster " + foreignClusterId)
	}

	sa, err := localClient.CoreV1().ServiceAccounts(pr.Spec.Namespace).Get(saName, metav1.GetOptions{})
	if err != nil {
		klog.Errorln(err, "Unable to get ServiceAccount "+saName)
		return err
	}

	// create the kubeconfig to allow the foreign cluster to create resources on local cluster
	kubeconfigForForeignCluster, err := kubeconfig.CreateKubeConfig(localClient, sa.Name, sa.Namespace)
	if err != nil {
		klog.Errorln(err, "Unable to create Kubeconfig")
		return err
	}
	// put the kubeconfig in a Secret, which is created on the foreign cluster
	kubeconfigSecretForForeign := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vk-kubeconfig-secret-" + homeClusterId,
			Namespace: sa.Namespace,
		},
		Data: nil,
		StringData: map[string]string{
			"kubeconfig": kubeconfigForForeignCluster,
		},
	}
	err = pkg.CreateOrUpdate(remoteClient, context.Background(), kubeconfigSecretForForeign)
	if err != nil {
		// secret not created, without it the vk cannot be launched: just klog and exit
		klog.Errorln(err, "Unable to create secret on remote cluster "+foreignClusterId)
		return err
	} else {
		// secret correctly created on foreign cluster, now create the Advertisement to trigger the creation of the virtual-kubelet
		broadcaster := AdvertisementBroadcaster{
			LocalClient:                localClient,
			LocalCRDClient:             localCRDClient,
			DiscoveryClient:            discoveryClient,
			KubeconfigSecretForForeign: kubeconfigSecretForForeign,
			RemoteClient:               remoteClient,
			HomeClusterId:              homeClusterId,
			ForeignClusterId:           pr.Name,
			GatewayIP:                  gatewayIP,
			GatewayPrivateIP:           gatewayPrivateIP,
		}

		broadcaster.WatchConfiguration(localKubeconfigPath)

		broadcaster.GenerateAdvertisement(foreignKubeconfigPath)
		// if we come here there has been an error while the broadcaster was running
		return errors.New("error while running Advertisement Broadcaster")
	}
}

// generate an advertisement message every 10 minutes and post it to remote clusters
// parameters
// - foreignKubeconfigPath: the path to a kubeconfig file. If set, this file is used to create a client to the foreign cluster. Set it only for debugging purposes
func (b *AdvertisementBroadcaster) GenerateAdvertisement(foreignKubeconfigPath string) {

	var once sync.Once

	for {
		// get physical and virtual nodes in the cluster
		physicalNodes, err := b.LocalClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "type != virtual-node"})
		if err != nil {
			klog.Errorln(err, "Could not get physical nodes, retry in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		virtualNodes, err := b.LocalClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "type = virtual-node"})
		if err != nil {
			klog.Errorln(err, "Could not get virtual nodes, retry in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		// get resources used by pods in the cluster
		fieldSelector, err := fields.ParseSelector("status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
		if err != nil {
			klog.Errorln(err, "Could not parse field selector")
			break
		}
		nodeNonTerminatedPodsList, err := b.LocalClient.CoreV1().Pods("").List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
		if err != nil {
			klog.Errorln(err, "Could not list pods, retry in 1 minute")
			time.Sleep(1 * time.Minute)
			continue
		}
		reqs, limits := GetAllPodsResources(nodeNonTerminatedPodsList)
		// compute resources to be announced to the other cluster
		availability, images := ComputeAnnouncedResources(physicalNodes, reqs, int64(b.ClusterConfig.ResourceSharingPercentage))

		// create the Advertisement on the foreign cluster
		adv := b.CreateAdvertisement(physicalNodes, virtualNodes, availability, images, limits)
		err = pkg.CreateOrUpdate(b.RemoteClient, context.Background(), &adv)
		if err != nil {
			klog.Errorln(err, "Unable to create advertisement on remote cluster "+b.ForeignClusterId)
		} else {
			// Advertisement created, set the owner reference of the secret so that it is deleted when the adv is removed
			klog.Info("Correctly created advertisement on remote cluster " + b.ForeignClusterId)
			adv.Kind = "Advertisement"
			adv.APIVersion = protocolv1.GroupVersion.String()
			b.KubeconfigSecretForForeign.SetOwnerReferences(pkg.GetOwnerReference(&adv))
			err = b.RemoteClient.Update(context.Background(), b.KubeconfigSecretForForeign)
			if err != nil {
				klog.Errorln(err, "Unable to update secret "+b.KubeconfigSecretForForeign.Name)
			}
			// start the remote watcher over this Advertisement; the watcher must be launched only once
			go once.Do(func() {
				scheme := runtime.NewScheme()
				_ = clientgoscheme.AddToScheme(scheme)
				_ = protocolv1.AddToScheme(scheme)
				WatchAdvertisement(b.LocalCRDClient, scheme, foreignKubeconfigPath, b.KubeconfigSecretForForeign, b.HomeClusterId, b.ForeignClusterId)
			})
		}
		time.Sleep(10 * time.Minute)
	}
}

// create advertisement message
func (b *AdvertisementBroadcaster) CreateAdvertisement(physicalNodes *corev1.NodeList, virtualNodes *corev1.NodeList,
	availability corev1.ResourceList, images []corev1.ContainerImage, limits corev1.ResourceList) protocolv1.Advertisement {

	// set prices field
	prices := ComputePrices(images)
	// use virtual nodes to build neighbours
	neighbours := make(map[corev1.ResourceName]corev1.ResourceList)
	for _, vnode := range virtualNodes.Items {
		neighbours[corev1.ResourceName(vnode.Name)] = vnode.Status.Allocatable
	}

	adv := protocolv1.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "advertisement-" + b.HomeClusterId,
			Namespace: "default",
		},
		Spec: protocolv1.AdvertisementSpec{
			ClusterId: b.HomeClusterId,
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
				GatewayIP:          b.GatewayIP,
				GatewayPrivateIP:   b.GatewayPrivateIP,
				SupportedProtocols: nil,
			},
			KubeConfigRef: corev1.ObjectReference{
				Kind:       b.KubeconfigSecretForForeign.Kind,
				Namespace:  b.KubeconfigSecretForForeign.Namespace,
				Name:       b.KubeconfigSecretForForeign.Name,
				UID:        b.KubeconfigSecretForForeign.UID,
				APIVersion: b.KubeconfigSecretForForeign.APIVersion,
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
	//TODO: get the node with the "gateway" label
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
func ComputeAnnouncedResources(physicalNodes *corev1.NodeList, reqs corev1.ResourceList, sharingPercentage int64) (availability corev1.ResourceList, images []corev1.ContainerImage) {
	// get allocatable resources in all the physical nodes
	allocatable, images := GetClusterResources(physicalNodes.Items)

	// subtract used resources from available ones to have available resources
	cpu := allocatable.Cpu().DeepCopy()
	cpu.Sub(reqs.Cpu().DeepCopy())
	mem := allocatable.Memory().DeepCopy()
	mem.Sub(reqs.Memory().DeepCopy())
	pods := allocatable.Pods().DeepCopy()

	// TODO: policy to decide how many resources to announce
	cpu.SetScaled(cpu.MilliValue()*sharingPercentage/100, resource.Milli)
	mem.Set(mem.Value() * sharingPercentage / 100)
	pods.Set(pods.Value() * sharingPercentage / 100)
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
