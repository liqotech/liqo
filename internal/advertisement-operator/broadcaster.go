package advertisementOperator

import (
	"context"
	"errors"
	"github.com/liqotech/liqo/internal/discovery/kubeconfig"
	pkg "github.com/liqotech/liqo/pkg"
	advpkg "github.com/liqotech/liqo/pkg/advertisement-operator"
	"github.com/liqotech/liqo/pkg/crdClient"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog"
	"strings"
	"sync"
	"time"

	configv1alpha1 "github.com/liqotech/liqo/api/config/v1alpha1"
	discoveryv1alpha1 "github.com/liqotech/liqo/api/discovery/v1alpha1"
	advtypes "github.com/liqotech/liqo/api/sharing/v1alpha1"
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
	PeeringRequestName string
	ClusterConfig      configv1alpha1.ClusterConfigSpec
	mutex              sync.Mutex
}

// start the broadcaster which sends Advertisement messages
// it reads the Secret to get the kubeconfig to the remote cluster and create a client for it
// parameters
// - homeClusterId: the cluster ID of your cluster (must be a UUID)
// - localKubeconfigPath: the path to the kubeconfig of the local cluster. Set it only when you are debugging and need to launch the program as a process and not inside Kubernetes
// - peeringRequestName: the name of the PeeringRequest containing the reference to the secret with the kubeconfig for creating Advertisements CR on foreign cluster
// - saName: The name of the ServiceAccount used to create the kubeconfig that will be sent to the foreign cluster with the permissions to create resources on local cluster
func StartBroadcaster(homeClusterId, localKubeconfigPath, peeringRequestName, saName string) error {
	klog.V(6).Info("starting broadcaster")

	// create the Advertisement client to the local cluster
	localClient, err := advtypes.CreateAdvertisementClient(localKubeconfigPath, nil)
	if err != nil {
		klog.Errorln(err, "Unable to create client to local cluster")
		return err
	}

	// create the discovery client
	config, err := crdClient.NewKubeconfig(localKubeconfigPath, &discoveryv1alpha1.GroupVersion)
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
	pr, ok := tmp.(*discoveryv1alpha1.PeeringRequest)
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
		remoteClient, err = advtypes.CreateAdvertisementClient("", secretForAdvertisementCreation)
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

	broadcaster := AdvertisementBroadcaster{
		LocalClient:        localClient,
		DiscoveryClient:    discoveryClient,
		RemoteClient:       remoteClient,
		HomeClusterId:      homeClusterId,
		ForeignClusterId:   pr.Name,
		PeeringRequestName: peeringRequestName,
	}

	kubeconfigSecretName := pkg.VirtualKubeletSecPrefix + homeClusterId
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
				Name:      kubeconfigSecretName,
				Namespace: sa.Namespace,
			},
			Data: nil,
			StringData: map[string]string{
				"kubeconfig": kubeconfigForForeignCluster,
			},
		}

		kubeconfigSecretForForeign, err = broadcaster.SendSecretToForeignCluster(kubeconfigSecretForForeign)
		if err != nil {
			// secret not created, without it the vk cannot be launched: just log and exit
			klog.Errorf("Unable to create secret for virtualKubelet on remote cluster %v; error: %v", foreignClusterId, err)
			return err
		}
	}
	// secret correctly created on foreign cluster, now launch the broadcaster to create Advertisement
	broadcaster.KubeconfigSecretForForeign = kubeconfigSecretForForeign

	broadcaster.WatchConfiguration(localKubeconfigPath, nil)

	broadcaster.GenerateAdvertisement()
	// if we come here there has been an error while the broadcaster was running
	return errors.New("error while running Advertisement Broadcaster")

}

// generate an Advertisement message every 10 minutes and post it to remote clusters
func (b *AdvertisementBroadcaster) GenerateAdvertisement() {

	var once sync.Once

	for {
		_, err := b.SendSecretToForeignCluster(b.KubeconfigSecretForForeign)
		if err != nil {
			klog.Errorln(err, "Error while sending Secret for virtual-kubelet to cluster "+b.ForeignClusterId)
			time.Sleep(1 * time.Minute)
			continue
		}

		_, virtualNodes, availability, limits, images, err := b.GetResourcesForAdv()
		if err != nil {
			klog.Errorln(err, "Error while computing resources for Advertisement")
			time.Sleep(1 * time.Minute)
			continue
		}

		// create the Advertisement on the foreign cluster
		advToCreate := b.CreateAdvertisement(virtualNodes, availability, images, limits)
		adv, err := b.SendAdvertisementToForeignCluster(advToCreate)
		if err != nil {
			klog.Errorln(err, "Error while sending Advertisement to cluster "+b.ForeignClusterId)
			time.Sleep(1 * time.Minute)
			continue
		}

		// start the remote watcher over this Advertisement; the watcher must be launched only once
		go once.Do(func() {
			b.WatchAdvertisement(adv.Name)
		})

		time.Sleep(10 * time.Minute)
	}
}

// create advertisement message
func (b *AdvertisementBroadcaster) CreateAdvertisement(virtualNodes *corev1.NodeList,
	availability corev1.ResourceList, images []corev1.ContainerImage, limits corev1.ResourceList) advtypes.Advertisement {

	// set prices field
	prices := ComputePrices(images)
	// use virtual nodes to build neighbours
	neighbours := make(map[corev1.ResourceName]corev1.ResourceList)
	for _, vnode := range virtualNodes.Items {
		neighbours[corev1.ResourceName(vnode.Name)] = vnode.Status.Allocatable
	}

	adv := advtypes.Advertisement{
		ObjectMeta: metav1.ObjectMeta{
			Name: pkg.AdvertisementPrefix + b.HomeClusterId,
		},
		Spec: advtypes.AdvertisementSpec{
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

func (b *AdvertisementBroadcaster) SendAdvertisementToForeignCluster(advToCreate advtypes.Advertisement) (*advtypes.Advertisement, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	var adv *advtypes.Advertisement

	// try to get the Advertisement on remote cluster
	obj, err := b.RemoteClient.Resource("advertisements").Get(advToCreate.Name, metav1.GetOptions{})
	if err == nil {
		// Advertisement already created, update it
		adv = obj.(*advtypes.Advertisement)
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
			return nil, err
		} else {
			// Advertisement created, set the owner reference of the secret so that it is deleted when the adv is removed
			adv = obj.(*advtypes.Advertisement)
			klog.Info("Correctly created advertisement on remote cluster " + b.ForeignClusterId)
			adv.Kind = "Advertisement"
			adv.APIVersion = advtypes.GroupVersion.String()
			b.KubeconfigSecretForForeign.SetOwnerReferences(advpkg.GetOwnerReference(adv))
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

func (b *AdvertisementBroadcaster) SendSecretToForeignCluster(secret *corev1.Secret) (*corev1.Secret, error) {
	secretForeign, err := b.RemoteClient.Client().CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
	if err == nil {
		// secret already created, update it
		secret.SetResourceVersion(secretForeign.ResourceVersion)
		secret.SetUID(secretForeign.UID)
		secretForeign, err = b.RemoteClient.Client().CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Unable to update secret %v on remote cluster %v; error: %v", secret.Name, b.ForeignClusterId, err)
			return nil, err
		}
		klog.Infof("Correctly updated secret %v on remote cluster %v", secret.Name, b.ForeignClusterId)
	} else if k8serrors.IsNotFound(err) {
		// secret not found, create it
		secretForeign, err = b.RemoteClient.Client().CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
		if err != nil {
			// secret not created, without it the vk cannot be launched: just log and exit
			klog.Errorf("Unable to create secret %v on remote cluster %v; error: %v", secret.Name, b.ForeignClusterId, err)
			return nil, err
		}
		klog.Infof("Correctly created secret %v on remote cluster %v", secret.Name, b.ForeignClusterId)
	} else {
		klog.Errorln("Unexpected error while getting Secret " + secret.Name)
		return nil, err
	}
	return secretForeign, nil
}

func (b *AdvertisementBroadcaster) NotifyAdvertisementDeletion() error {
	advName := pkg.AdvertisementPrefix + b.HomeClusterId
	// delete adv to inform the vk to do the cleanup
	err := b.RemoteClient.Resource("advertisements").Delete(advName, metav1.DeleteOptions{})
	if err != nil {
		klog.Error("Unable to delete Advertisement " + advName)
		return err
	}
	return nil
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
	m := make(map[string]corev1.ContainerImage)

	for _, image := range node.Status.Images {
		for _, name := range image.Names {
			// TODO: policy to decide which images to announce
			if !strings.Contains(name, "k8s") {
				m[name] = image
			}
		}
	}
	images := make([]corev1.ContainerImage, len(m))
	i := 0
	for _, image := range m {
		images[i] = image
		i++
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
