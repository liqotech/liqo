package main

import (
	"drone_daemon_resources/configuration"
	"drone_daemon_resources/messaging"
	"encoding/json"
	"flag"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/kubernetes"
)

var configurationEnv *configuration.ConfigType

var rabbit *messaging.RabbitMq

var clientSet *kubernetes.Clientset

// Resources for cluster
type ClusterResources struct {
	CPU    resource.Quantity
	Memory resource.Quantity
	Pods   resource.Quantity
}

// PodMetricsList : PodMetricsList
type PodMetricsList struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink string `json:"selfLink"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Name              string    `json:"name"`
			Namespace         string    `json:"namespace"`
			SelfLink          string    `json:"selfLink"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Timestamp  time.Time `json:"timestamp"`
		Window     string    `json:"window"`
		Containers []struct {
			Name  string `json:"name"`
			Usage struct {
				CPU    resource.Quantity `json:"cpu"`
				Memory resource.Quantity `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

// PodMetricsList : NodeMetricsList
type NodeMetricsList struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Metadata   struct {
		SelfLink string `json:"selfLink"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
			Name              string    `json:"name"`
			Namespace         string    `json:"namespace"`
			SelfLink          string    `json:"selfLink"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
		} `json:"metadata"`
		Timestamp time.Time `json:"timestamp"`
		Window    string    `json:"window"`
		Usage     struct {
			CPU    resource.Quantity `json:"cpu"`
			Memory resource.Quantity `json:"memory"`
			Pod    resource.Quantity `json:"pod"`
		} `json:"usage"`
	} `json:"items"`
}

//creates the in-cluster configuration
func createInClusterConfiguration() *rest.Config {
	confCluster, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	return confCluster
}

//creates the out-cluster configuration
func createOutClusterConfiguration() *rest.Config {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()

	// use the current context in kubeconfig
	confCluster, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	return confCluster
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func setLog() {
	Formatter := new(log.TextFormatter)
	Formatter.TimestampFormat = "02-01-2006 15:04:05"
	Formatter.FullTimestamp = true
	Formatter.ForceColors = true
	log.SetFormatter(Formatter)
	log.SetLevel(log.DebugLevel)
}

func main() {

	// Create cluster config
	config := createInClusterConfiguration()
	//config := createOutClusterConfiguration()

	// Setting fo log module
	setLog()

	// Load and create configurationEnv
	configurationEnv = configuration.Config()

	// creates the clientSet
	var err error
	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// Init rabbitMq connection, channel and queue
	rabbit = messaging.InitRabbitMq(configurationEnv.RabbitConf.QueueResources,
									configurationEnv.RabbitConf.BrokerAddress,
									configurationEnv.RabbitConf.BrokerPort,
									configurationEnv.RabbitConf.VirtualHost,
									configurationEnv.RabbitConf.Username,
									configurationEnv.RabbitConf.Password)

	// Sent first resources
	err = createAndSendMessage(clientSet)
	if err != nil {
		panic(err.Error())
	}

	// Create a cache to store Pods
	var podsStore cache.Store
	// Watch for Pods Change.....
	podsStore = watchPods(clientSet, podsStore)
	// Keep alive
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func podCreated(obj interface{}) {
	pod := obj.(*v1.Pod)
	log.Debug("Pod created: " + pod.ObjectMeta.Name)
	err := createAndSendMessage(clientSet)
	if err != nil {
		log.Error(err)
	}
}

func podDeleted(obj interface{}) {
	pod := obj.(*v1.Pod)
	log.Debug("Pod deleted: " + pod.ObjectMeta.Name)
	err := createAndSendMessage(clientSet)
	if err != nil {
		log.Error(err)
	}
}

func podUpdated(oldObj, obj interface{}) {
	pod := obj.(*v1.Pod)
	podOld := oldObj.(*v1.Pod)
	log.Debug("Pod updated: " + pod.ObjectMeta.Name + " old: " + podOld.ObjectMeta.Name)
}

func watchPods(clientSet *kubernetes.Clientset, store cache.Store) cache.Store {
	//Define what we want to look for (Pods)
	// v1.NamespaceAll
	watchlist := cache.NewListWatchFromClient(clientSet.CoreV1().RESTClient(), "pods", configurationEnv.Kubernetes.Namespace, fields.Everything())
	resyncPeriod := 30 * time.Minute
	//Setup an informer to call functions when the watchlist changes
	eStore, eController := cache.NewInformer(
		watchlist,
		&v1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    podCreated,
			DeleteFunc: podDeleted,
			UpdateFunc: podUpdated,
		},
	)
	//Run the controller as a goroutine
	go eController.Run(wait.NeverStop)
	return eStore
}

func createAndSendMessage(clientSet *kubernetes.Clientset) error {
	// Get cluster allocatable resources
	var clusterAllocatable ClusterResources
	err := getClusterResourcesAllocatable(clientSet, &clusterAllocatable)
	if err != nil {
		log.Println(err)
		return err
	}

	// Calculate total cluster available (for example remove pod default)
	var totalClusterResources = calculateTotalClusterResources(clientSet, clusterAllocatable)

	// Get cluster used resources
	var clusterUsed ClusterResources
	err = getClusterResourcesUsed(clientSet, &clusterUsed, configurationEnv.Kubernetes.Namespace)
	if err != nil {
		log.Error(err)
		return err
	}

	// Calculate cpu and memory free and scale
	var memory = ((float64(totalClusterResources.Memory.Value()) - float64(clusterUsed.Memory.Value())) / 1024 / 1024)*(float64(configurationEnv.Resources.Scale)/100)
	var cpu = ((float64(totalClusterResources.CPU.MilliValue()) - float64(clusterUsed.CPU.MilliValue())) / 1000)*(float64(configurationEnv.Resources.Scale)/100)

	// Create new message
	message := messaging.NewResourceMessage(configurationEnv.Kubernetes.ClusterName, memory, cpu)
	//log.Info(" Created Message %s", message)

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Println(err)
	}
	log.Debug(" Json Message: ", string(jsonData))

	// Publish message
	rabbit.PublishMessage(string(jsonData))

	return err
}

func calculateTotalClusterResources(clientSet *kubernetes.Clientset, resourcesAllocatable ClusterResources) ClusterResources {
	//var total ClusterResources

	var podsKubeSystem v1.PodList
	err := getAllPodsNamespaced(clientSet, &podsKubeSystem, "kube-system")
	if err != nil {
		panic(err.Error())
	}

	for _, p := range podsKubeSystem.Items {
		for _, c := range p.Spec.Containers {
			resourcesAllocatable.Memory.Sub(c.Resources.Requests.Memory().DeepCopy())
			resourcesAllocatable.CPU.Sub(c.Resources.Requests.Cpu().DeepCopy())
		}
	}

	return resourcesAllocatable
}

// In docker set only one node, cluster is host machine
func getClusterResourcesAllocatable(clientSet *kubernetes.Clientset, clusterAllocatable *ClusterResources) error {
	nodes, err := clientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	for _, n := range nodes.Items {
		var _noSchedule = false
		// Check if node is not able for schedule
		if n.Spec.Taints != nil {
			for _, t := range n.Spec.Taints {
				if t.Effect == v1.TaintEffectNoSchedule {
					log.Printf(" %s --> noSchedule node", n.Name)
					_noSchedule = true
					break
				}
			}
		}
		// Nodes can be considered for schedule job
		if _noSchedule != true {
			log.Println(" - ", n.Name, n.Status.Allocatable.Cpu(), n.Status.Allocatable.Memory(), n.Status.Allocatable.Pods())
			clusterAllocatable.Pods.Add(*n.Status.Allocatable.Pods())
			clusterAllocatable.CPU.Add(*n.Status.Allocatable.Cpu())
			clusterAllocatable.Memory.Add(*n.Status.Allocatable.Memory())
		}
	}
	return err
}

// memory resource.quantity --> memory.Value()/1024/1024 	(return memory value in Mi)
// cpu resource.quantity --> cpu.MilliValue()		(return cpu value in m)

func getClusterResourcesUsed(clientSet *kubernetes.Clientset, clusterUsed *ClusterResources, namespace string) error {

	var resourceTmp ClusterResources

	var pods v1.PodList
	err := getAllPodsNamespaced(clientSet, &pods, namespace)
	if err != nil {
		return err
	}
	for _, p := range pods.Items {
		for _, c := range p.Spec.Containers {
			resourceTmp.Memory.Add(c.Resources.Limits.Memory().DeepCopy())
			resourceTmp.CPU.Add(c.Resources.Limits.Cpu().DeepCopy())
		}
	}

	/*var pods PodMetricsList
	err := getMetricsPodsNamespaced(clientSet, &pods, namespace)
	if err != nil {
		return err
	}
	for _, p := range pods.Items {
		for _, c := range p.Containers {
			resourceTmp.Memory.Add(c.Usage.Memory)
			resourceTmp.CPU.Add(c.Usage.CPU)
		}
	}
	*clusterUsed = resourceTmp*/

	*clusterUsed = resourceTmp

	return err
}

// Get Pods metric, use metrics-service
func getMetricsPods(clientSet *kubernetes.Clientset, pods *PodMetricsList) error {
	data, err := clientSet.RESTClient().Get().AbsPath("apis/metrics.k8s.io/v1beta1/pods").DoRaw()
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &pods)
	return err
}

// Get Pods metric namespaced, use metrics-service
func getMetricsPodsNamespaced(clientSet *kubernetes.Clientset, pods *PodMetricsList, namespace string) error {
	data, err := clientSet.RESTClient().Get().AbsPath("apis/metrics.k8s.io/v1beta1/namespaces/" + namespace + "/pods").DoRaw()
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &pods)
	return err
}

// Get Nodes metric, use metrics-service
func getMetricsNodes(clientSet *kubernetes.Clientset, nodes *NodeMetricsList) error {
	data, err := clientSet.RESTClient().Get().AbsPath("apis/metrics.k8s.io/v1beta1/nodes").DoRaw()
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &nodes)
	return err
}

// Get Pods in all namespace
func getAllPods(clientSet *kubernetes.Clientset, pods *v1.PodList) error {
	podsTmp, err := clientSet.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	*pods = *podsTmp

	return err
}

// Get Pods in particular namespace
func getAllPodsNamespaced(clientSet *kubernetes.Clientset, pods *v1.PodList, namespace string) error {

	podsTmp, err := clientSet.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	*pods = *podsTmp

	return err
}

// Get Pods in all namespace
func getAllNodes(clientSet *kubernetes.Clientset, nodes *v1.NodeList) error {
	nodesTmp, err := clientSet.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	*nodes = *nodesTmp

	return err
}
