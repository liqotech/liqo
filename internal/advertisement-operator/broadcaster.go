package advertisement_operator

import (
	"context"
	"errors"
	"github.com/liqoTech/liqo/internal/discovery/kubeconfig"
	pkg "github.com/liqoTech/liqo/pkg/advertisement-operator"
	"github.com/liqoTech/liqo/pkg/crdClient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog"
	"strings"
	"sync"
	"time"

	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	discoveryv1 "github.com/liqoTech/liqo/api/discovery/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	resourcehelper "k8s.io/kubectl/pkg/util/resource"
)

type AdvertisementBroadcaster struct {
	// local-related variables
	LocalClient     *crdClient.CRDClient
	DiscoveryClient *crdClient.CRDClient
	// remote-related variables
	KubeconfigSecretForForeign *corev1.Secret       // secret containing the kubeconfig that will be sent to the foreign cluster
	RemoteClient               *crdClient.CRDClient // client to create Advertisements and Secrets on the foreign cluster
	// configuration variables
	HomeClusterId      string
	ForeignClusterId   string
	GatewayPrivateIP   string
	PeeringRequestName string
	ClusterConfig      policyv1.ClusterConfigSpec
}

// start the broadcaster which sends Advertisement messages
// it reads the Secret to get the kubeconfig to the remote cluster and create a client for it
// parameters
// - homeClusterId: the cluster ID of your cluster (must be a UUID)
// - localKubeconfigPath: the path to the kubeconfig of the local cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - gatewayPrivateIP: the private IP address of the gateway node
// - peeringRequestName: the name of the PeeringRequest containing the reference to the secret with the kubeconfig for creating Advertisements CR on foreign cluster
// - saName: The name of the ServiceAccount used to create the kubeconfig that will be sent to the foreign cluster with the permissions to create resources on local cluster
func StartBroadcaster(homeClusterId, localKubeconfigPath, gatewayPrivateIP, peeringRequestName, saName string) error {
	klog.V(6).Info("starting broadcaster")

	// create the Advertisement client to the local cluster
	localClient, err := protocolv1.CreateAdvertisementClient(localKubeconfigPath, nil)
	if err != nil {
		klog.Errorln(err, "Unable to create client to local cluster")
		return err
	}

	// create the discovery client
	config, err := crdClient.NewKubeconfig(localKubeconfigPath, &discoveryv1.GroupVersion)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}
	discoveryClient, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, err.Error())
		return err
	}

	// get the PeeringRequest from the foreign cluster which requested resources
	tmp, err := discoveryClient.Resource("peeringrequests").Get(peeringRequestName, metav1.GetOptions{})
	if err != nil {
		klog.Errorln(err, "Unable to get PeeringRequest "+peeringRequestName)
		return err
	}
	pr, ok := tmp.(*discoveryv1.PeeringRequest)
	if !ok {
		return errors.New("retrieved object is not a PeeringRequest")
	}

	foreignClusterId := pr.Name

	// get the Secret with the permission to create Advertisements and Secrets on foreign cluster
	secretForAdvertisementCreation, err := localClient.Client().CoreV1().Secrets(pr.Spec.KubeConfigRef.Namespace).Get(context.TODO(), pr.Spec.KubeConfigRef.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorln(err, "Unable to get PeeringRequest secret")
		return err
	}

	// create the Advertisement client to the remote cluster, using the retrieved Secret
	var remoteClient *crdClient.CRDClient
	var retry int

	// create a CRD-client to the foreign cluster
	for retry = 0; retry < 3; retry++ {
		remoteClient, err = protocolv1.CreateAdvertisementClient("", secretForAdvertisementCreation)
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

	kubeconfigSecretName := "vk-kubeconfig-secret-" + homeClusterId
	kubeconfigSecretForForeign, err := remoteClient.Client().CoreV1().Secrets(pr.Spec.Namespace).Get(context.TODO(), kubeconfigSecretName, metav1.GetOptions{})
	if err != nil {
		// secret containing kubeconfig not found: create it
		// get the ServiceAccount with the permissions that will be given to the foreign cluster
		sa, err := localClient.Client().CoreV1().ServiceAccounts(pr.Spec.Namespace).Get(context.TODO(), saName, metav1.GetOptions{})
		if err != nil {
			klog.Errorln(err, "Unable to get ServiceAccount "+saName)
			return err
		}

		// create the kubeconfig to allow the foreign cluster to create resources on local cluster
		kubeconfigForForeignCluster, err := kubeconfig.CreateKubeConfig(localClient.Client(), sa.Name, sa.Namespace)
		if err != nil {
			klog.Errorln(err, "Unable to create Kubeconfig")
			return err
		}
		// put the kubeconfig in a Secret, which is created on the foreign cluster
		kubeconfigSecretForForeign = &corev1.Secret{
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
		kubeconfigSecretForForeign, err = remoteClient.Client().CoreV1().Secrets(sa.Namespace).Create(context.TODO(), kubeconfigSecretForForeign, metav1.CreateOptions{})
		if err != nil {
			// secret not created, without it the vk cannot be launched: just log and exit
			klog.Errorln(err, "Unable to create secret on remote cluster "+foreignClusterId)
			return err
		}
	}
	// secret correctly created on foreign cluster, now create the Advertisement to trigger the creation of the virtual-kubelet
	broadcaster := AdvertisementBroadcaster{
		LocalClient:                localClient,
		DiscoveryClient:            discoveryClient,
		KubeconfigSecretForForeign: kubeconfigSecretForForeign,
		RemoteClient:               remoteClient,
		HomeClusterId:              homeClusterId,
		ForeignClusterId:           pr.Name,
		GatewayPrivateIP:           gatewayPrivateIP,
		PeeringRequestName:         peeringRequestName,
	}

	broadcaster.WatchConfiguration(localKubeconfigPath, nil)

	broadcaster.GenerateAdvertisement()
	// if we come here there has been an error while the broadcaster was running
	return errors.New("error while running Advertisement Broadcaster")

}

// generate an Advertisement message every 10 minutes and post it to remote clusters
func (b *AdvertisementBroadcaster) GenerateAdvertisement() {

	var once sync.Once

	for {
		physicalNodes, virtualNodes, availability, limits, images, err := b.GetResourcesForAdv()
		if err != nil {
			klog.Errorln(err, "Error while computing resources for Advertisement")
			time.Sleep(1 * time.Minute)
			continue
		}

		// create the Advertisement on the foreign cluster
		advToCreate := b.CreateAdvertisement(physicalNodes, virtualNodes, availability, images, limits)
		adv, err := b.SendAdvertisementToForeignCluster(advToCreate)
		if err != nil {
			klog.Errorln(err, "Error while sending Advertisement to cluster "+b.ForeignClusterId)
			continue
		}

		// start the remote watcher over this Advertisement; the watcher must be launched only once
		go once.Do(func() {
			foreignAdvName := "advertisement-" + b.ForeignClusterId
			b.WatchAdvertisement(adv.Name, foreignAdvName)
		})

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
			Name: "advertisement-" + b.HomeClusterId,
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
				GatewayIP:          GetGateway(physicalNodes.Items),
				GatewayPrivateIP:   b.GatewayPrivateIP,
				SupportedProtocols: nil,
			},
			KubeConfigRef: corev1.SecretReference{
				Namespace: b.KubeconfigSecretForForeign.Namespace,
				Name:      b.KubeconfigSecretForForeign.Name,
			},
			Timestamp:  metav1.NewTime(time.Now()),
			TimeToLive: metav1.NewTime(time.Now().Add(30 * time.Minute)),
		},
	}
	return adv
}

func (b *AdvertisementBroadcaster) GetResourcesForAdv() (physicalNodes, virtualNodes *corev1.NodeList, availability, limits corev1.ResourceList, images []corev1.ContainerImage, err error) {
	// get physical and virtual nodes in the cluster
	physicalNodes, err = b.LocalClient.Client().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "type != virtual-node"})
	if err != nil {
		klog.Errorln("Could not get physical nodes, retry in 1 minute")
		return nil, nil, nil, nil, nil, err
	}
	virtualNodes, err = b.LocalClient.Client().CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "type = virtual-node"})
	if err != nil {
		klog.Errorln("Could not get virtual nodes, retry in 1 minute")
		return nil, nil, nil, nil, nil, err
	}
	// get resources used by pods in the cluster
	fieldSelector, err := fields.ParseSelector("status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	nodeNonTerminatedPodsList, err := b.LocalClient.Client().CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{FieldSelector: fieldSelector.String()})
	if err != nil {
		klog.Errorln("Could not list pods, retry in 1 minute")
		return nil, nil, nil, nil, nil, err
	}
	reqs, limits := GetAllPodsResources(nodeNonTerminatedPodsList)
	// compute resources to be announced to the other cluster
	availability, images = ComputeAnnouncedResources(physicalNodes, reqs, int64(b.ClusterConfig.AdvertisementConfig.OutgoingConfig.ResourceSharingPercentage))

	return physicalNodes, virtualNodes, availability, limits, images, nil
}

func (b *AdvertisementBroadcaster) SendAdvertisementToForeignCluster(advToCreate protocolv1.Advertisement) (*protocolv1.Advertisement, error) {
	var adv *protocolv1.Advertisement

	// try to get the Advertisement on remote cluster
	obj, err := b.RemoteClient.Resource("advertisements").Get(advToCreate.Name, metav1.GetOptions{})
	if err == nil {
		// Advertisement already created, update it
		adv = obj.(*protocolv1.Advertisement)
		advToCreate.SetResourceVersion(adv.ResourceVersion)
		advToCreate.SetUID(adv.UID)
		_, err = b.RemoteClient.Resource("advertisements").Update(adv.Name, &advToCreate, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorln("Unable to update Advertisement " + advToCreate.Name)
			return nil, err
		}
	} else if k8serrors.IsNotFound(err) {
		// Advertisement not found, create it
		obj, err := b.RemoteClient.Resource("advertisements").Create(&advToCreate, metav1.CreateOptions{})
		if err != nil {
			klog.Errorln("Unable to create Advertisement " + advToCreate.Name + " on remote cluster " + b.ForeignClusterId)
			// clean remote cluster from the secret previously created for the adv
			if err := b.RemoteClient.Client().CoreV1().Secrets(b.KubeconfigSecretForForeign.Namespace).Delete(context.TODO(), b.KubeconfigSecretForForeign.Name, metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
			return nil, err
		} else {
			// Advertisement created, set the owner reference of the secret so that it is deleted when the adv is removed
			adv = obj.(*protocolv1.Advertisement)
			klog.Info("Correctly created advertisement on remote cluster " + b.ForeignClusterId)
			adv.Kind = "Advertisement"
			adv.APIVersion = protocolv1.GroupVersion.String()
			b.KubeconfigSecretForForeign.SetOwnerReferences(pkg.GetOwnerReference(adv))
			_, err = b.RemoteClient.Client().CoreV1().Secrets(b.KubeconfigSecretForForeign.Namespace).Update(context.TODO(), b.KubeconfigSecretForForeign, metav1.UpdateOptions{})
			if err != nil {
				klog.Errorln(err, "Unable to update secret "+b.KubeconfigSecretForForeign.Name)
			}
		}
	} else {
		klog.Errorln("Unexpected error while getting Advertisement " + advToCreate.Name)
		return nil, err
	}
	return adv, nil
}

func (b *AdvertisementBroadcaster) NotifyAdvertisementDeletion() error {
	advName := "advertisement-" + b.HomeClusterId
	obj, err := b.RemoteClient.Resource("advertisements").Get(advName, metav1.GetOptions{})
	if err != nil {
		klog.Error("Advertisement " + advName + " doesn't exist on foreign cluster " + b.ForeignClusterId)
	} else {
		// update the status of adv to inform the vk it is going to be deleted
		adv := obj.(*protocolv1.Advertisement)
		adv.Status.AdvertisementStatus = AdvertisementDeleting
		_, err = b.RemoteClient.Resource("advertisements").UpdateStatus(adv.Name, adv, metav1.UpdateOptions{})
		if err != nil {
			klog.Error("Unable to update Advertisement " + adv.Name)
			return err
		}
	}
	return nil
}

func GetPodCIDR(nodes []corev1.Node) string {
	var podCIDR string
	token := strings.Split(nodes[0].Spec.PodCIDR, ".")
	if len(token) >= 2 {
		podCIDR = token[0] + "." + token[1] + "." + "0" + "." + "0/16"
	} else {
		// in some cases (e.g. minikube) node PodCIDR is null, set a default one
		podCIDR = "172.17.0.0/16"
	}
	return podCIDR
}

func GetGateway(nodes []corev1.Node) string {
	for _, node := range nodes {
		if node.Labels["liqonet.liqo.io/gateway"] != "" {
			return node.Status.Addresses[0].Address
		}
	}
	// node with required label not found, return the first one
	return nodes[0].Status.Addresses[0].Address
}

// get resources used by pods on physical nodes
func GetAllPodsResources(nodeNonTerminatedPodsList *corev1.PodList) (requests corev1.ResourceList, limits corev1.ResourceList) {
	// remove pods on virtual nodes
	for i, pod := range nodeNonTerminatedPodsList.Items {
		if strings.HasPrefix(pod.Spec.NodeName, "liqo-") {
			nodeNonTerminatedPodsList.Items[i] = corev1.Pod{}
		}
	}
	requests, limits = getPodsTotalRequestsAndLimits(nodeNonTerminatedPodsList)
	return requests, limits
}

func getPodsTotalRequestsAndLimits(podList *corev1.PodList) (reqs map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity) {
	reqs, limits = map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for i := range podList.Items {
		pod := podList.Items[i]
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
	if cpu.Value() < 0 {
		cpu.Set(0)
	}
	mem := allocatable.Memory().DeepCopy()
	if mem.Value() < 0 {
		mem.Set(0)
	}
	mem.Sub(reqs.Memory().DeepCopy())
	pods := allocatable.Pods().DeepCopy()

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
