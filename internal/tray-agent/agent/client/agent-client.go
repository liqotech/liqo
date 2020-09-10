package client

import (
	"errors"
	"flag"
	"github.com/gen2brain/dlgs"
	"github.com/liqotech/liqo/api/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"sync"
)

const (
	//EnvLiqoKConfig defines the env var containing the path of the kubeconfig file of the
	//cluster associated to Liqo Agent.
	EnvLiqoKConfig = "LIQO_KCONFIG"
)

//AgentController singleton.
var agentCtrl *AgentController

//mockedController controls if the AgentController has to be mocked.
var mockedController bool

//mockOnce prevents mockedController to be modified at runtime.
var mockOnce sync.Once

//UseMockedAgentController enables a mocked AgentController that does not interacts
//with the kubernetes cluster.
//
//Function MUST be called before GetAgentController in order to be effective.
func UseMockedAgentController() {
	mockOnce.Do(func() {
		mockedController = true
	})
}

//DestroyMockedAgentController destroys the AgentController singleton for
//testing purposes. It works only after calling UseMockedAgentController().
func DestroyMockedAgentController() {
	if mockedController {
		agentCtrl = nil
	}
}

//AgentController is the data structure that manages Tray Agent interaction with the cluster.
type AgentController struct {
	kubeClient kubernetes.Interface
	client     *crdClient.CRDClient
	valid      bool
	connected  bool
	advCache   *AdvertisementCache
	mocked     bool
}

//KubeClient returns a Kubernetes client to interact with the cluster.
func (ctrl *AgentController) KubeClient() kubernetes.Interface {
	return ctrl.kubeClient
}

//Mocked returns if the AgentController is mocked (true).
func (ctrl *AgentController) Mocked() bool {
	return ctrl.mocked
}

//Client returns the controller Client used for cluster interaction.
func (ctrl *AgentController) Client() *crdClient.CRDClient {
	return ctrl.client
}

//Connected returns if the controller client is actually connected to the cluster.
func (ctrl *AgentController) Connected() bool {
	if mockedController {
		return true
	}
	return ctrl.connected
}

//AdvCache returns the Advertisement cache of the controller.
func (ctrl *AgentController) AdvCache() *AdvertisementCache {
	return ctrl.advCache
}

//createClient returns a client for the Tray Agent that can operate on some Liqo CRD.
func createClient() (*crdClient.CRDClient, error) {
	var config *rest.Config
	var err error

	if err = v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
	kubePath, ok := os.LookupEnv("LIQO_KCONFIG")
	if !ok {
		if config := AcquireKubeconfig(); !config {
			return nil, err
		}
		kubePath = os.Getenv("LIQO_KCONFIG")
	}
	if _, err := os.Stat(kubePath); os.IsNotExist(err) {
		return nil, err
	}
	config, err = crdClient.NewKubeconfig(kubePath, &v1alpha1.GroupVersion)
	if err != nil {
		return nil, err
	}
	clientSet, err := crdClient.NewFromConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

//StartCaches starts all the CR caches of the AgentController.
func (ctrl *AgentController) StartCaches() {
	_ = ctrl.advCache.StartCache(ctrl.client)
}

//StopCaches stops all the CR caches running for the AgentController.
func (ctrl *AgentController) StopCaches() {
	ctrl.advCache.StopCache()
}

//GetAgentController returns an initialized AgentController singleton.
func GetAgentController() *AgentController {
	if agentCtrl == nil {
		agentCtrl = &AgentController{}
		agentCtrl.mocked = mockedController
		agentCtrl.advCache = createAdvCache()
		crdClient.AddToRegistry("advertisements", &v1alpha1.Advertisement{}, &v1alpha1.AdvertisementList{}, advertisementKeyer, v1alpha1.GroupResource)
		var err error
		if agentCtrl.kubeClient, err = createKubeClient(); err == nil {
			if !mockedController {
				if agentCtrl.client, err = createClient(); err == nil {
					agentCtrl.valid = true
					if test := agentCtrl.ConnectionTest(); test {
						agentCtrl.StartCaches()
					}
				}
			} else {
				agentCtrl.valid = true
				agentCtrl.connected = true
				agentCtrl.StartCaches()
			}
		}
	}
	return agentCtrl
}

// key extractor for the Advertisement CRD
func advertisementKeyer(obj runtime.Object) (string, error) {
	adv, ok := obj.(*v1alpha1.Advertisement)
	if !ok {
		return "", errors.New("cannot cast received object to Advertisement")
	}
	return adv.Name, nil
}

//ConnectionTest tests if the AgentController can connect to the cluster.
func (ctrl *AgentController) ConnectionTest() bool {
	if !ctrl.valid {
		return false
	}
	if _, err := ctrl.client.Resource("advertisements").List(metav1.ListOptions{}); err == nil {
		ctrl.connected = true
	} else {
		ctrl.connected = false
	}
	return ctrl.connected
}

// AcquireKubeconfig sets the LIQO_KCONFIG env variable.
// LIQO_KCONFIG represents the location of a kubeconfig file in order to let
// the client connect to the local cluster.
//
// The file path - if not expressed with the 'kubeconf' program argument -
// is set to $HOME/.kube/config .
//
// It returns whether a valid file path for a possible kubeconfig has been set.
func AcquireKubeconfig() bool {
	kubePath := filepath.Join(os.Getenv("HOME"), ".kube")
	kubeconfig := flag.String("kubeconf", filepath.Join(kubePath, "config"),
		"[OPT] absolute path to the kubeconfig file."+
			" Default = $HOME/.kube/config")
	flag.Parse()
	if err := os.Setenv(EnvLiqoKConfig, *kubeconfig); err != nil {
		panic(err)
	}
	if _, err := os.Stat(*kubeconfig); os.IsNotExist(err) {
		ok, _ := dlgs.Question("NO VALID KUBECONFIG FILE FOUND", "Liqo could not find a valid kubeconfig file.\n "+
			"Do you want to select a file?", false)
		if ok {
			path, selected, _ := dlgs.File("Select file", "", false)
			if selected {
				if err = os.Setenv(EnvLiqoKConfig, path); err != nil {
					panic(err)
				}
				return true
			}
		}
		return false
	}
	return true
}

//createKubeClient creates a new out-of-cluster client from a kubeconfig file.
//If no value for kubeconfig is provided, it returns an error.
//
//The file path is retrieved from the env var specified by EnvLiqoKConfig.
func createKubeClient() (kubernetes.Interface, error) {
	if mockedController {
		return fake.NewSimpleClientset(), nil
	}
	kubeconfig, ok := os.LookupEnv(EnvLiqoKConfig)
	if !ok || kubeconfig == "" {
		return nil, errors.New("no kubeconfig provided")
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
