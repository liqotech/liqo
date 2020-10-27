package client

import (
	"context"
	"errors"
	"flag"
	"github.com/gen2brain/dlgs"
	"github.com/liqotech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
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
		crdClient.Fake = true
	})
}

//DestroyMockedAgentController destroys the AgentController singleton for
//testing purposes. It works only after calling UseMockedAgentController().
func DestroyMockedAgentController() {
	if mockedController {
		agentCtrl = nil
	}
}

//kubeconfArg specifies the resulting value of the 'kubeconf' program argument after arguments parsing.
var kubeconfArg *string

//flagOnce prevents the program arguments flag redefinition which would cause panic.
var flagOnce sync.Once

//AgentController is the data structure that manages Tray Agent interaction with the cluster.
type AgentController struct {
	//notifyChannels is a set of channels used by the cache logic to notify a watched event.
	notifyChannels map[NotifyChannel]chan string
	//kubeClient is a standard kubernetes client.
	kubeClient kubernetes.Interface
	//crdManager that manages CRD operations.
	*crdManager
	//valid specifies whether the provided kubeconfig actually describes a correct configuration.
	valid bool
	//connected specifies whether all AgentController components are correctly up and running.
	connected bool
	mocked    bool
}

//Mocked returns if the AgentController is mocked (true).
func (ctrl *AgentController) Mocked() bool {
	return ctrl.mocked
}

//Connected returns if the Controller client is actually connected to the cluster.
func (ctrl *AgentController) Connected() bool {
	return ctrl.connected
}

//NotifyChannel returns the NotifyChannel of type 'channelType'.
func (ctrl *AgentController) NotifyChannel(channelType NotifyChannel) chan string {
	return ctrl.notifyChannels[channelType]
}

//StartCaches starts each available AgentController cache.
func (ctrl *AgentController) StartCaches() error {
	for _, crdCtrl := range ctrl.crdManager.clientMap {
		if err := crdCtrl.StartCache(); err != nil {
			return err
		}
	}
	return nil
}

//StopCaches stops all the CR caches running for the AgentController.
func (ctrl *AgentController) StopCaches() {
	for _, crdCtrl := range ctrl.crdManager.clientMap {
		crdCtrl.StopCache()
	}
}

/*acquireKubeconfig sets the LIQO_KCONFIG env variable.
EnvLiqoKConfig represents the path of a kubeconfig file required to let
the client connect to the local cluster.

- As first option, the function uses the file path of the 'kubeconf' program argument.

- If that argument is not provided, it defaults to $HOME/.kube/config.

- If the resulting filepath does not exist, users are asked to manually
select a valid one. Otherwise, EnvLiqoKConfig is not set.
*/
func acquireKubeconfig() {
	var path string
	var found bool
	if err := os.Unsetenv(EnvLiqoKConfig); err != nil {
		panic(err)
	}
	if mockedController {
		path = "/test/path"
		found = true
	} else {
		kubePath := filepath.Join(os.Getenv("HOME"), ".kube")
		flagOnce.Do(func() {
			kubeconfArg = flag.String("kubeconf", filepath.Join(kubePath, "config"),
				"[OPT] absolute path to the kubeconfig file."+
					" Default = $HOME/.kube/config")
			flag.Parse()
		})
		if _, err := os.Stat(*kubeconfArg); os.IsNotExist(err) {
			ok, _ := dlgs.Question("NO VALID KUBECONFIG FILE FOUND",
				"Liqo could not find a valid kubeconfig file.\n "+
					"Do you want to select one?", false)
			if ok {
				filePath, selected, _ := dlgs.File("Select file", "", false)
				if selected {
					path = filePath
					found = true
				}
			}
		} else {
			path = *kubeconfArg
			found = true
		}
	}
	if found {
		if err := os.Setenv(EnvLiqoKConfig, path); err != nil {
			panic(err)
		}
	}
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

//GetAgentController returns an initialized AgentController singleton.
func GetAgentController() *AgentController {
	if agentCtrl == nil {
		agentCtrl = &AgentController{}
		agentCtrl.mocked = mockedController
		//init the notifyChannels that are kept open during the entire Agent execution.
		agentCtrl.notifyChannels = make(map[NotifyChannel]chan string)
		for _, i := range notifyChannelNames {
			agentCtrl.notifyChannels[i] = make(chan string, notifyBuffLength)
		}
		var err error
		//acquire configuration, try to connect clients, start caches.
		acquireKubeconfig()
		if agentCtrl.kubeClient, err = createKubeClient(); err == nil {
			if err = agentCtrl.initCRDManager(); err == nil {
				if agentCtrl.ConnectionTest() {
					if err = agentCtrl.StartCaches(); err == nil {
						agentCtrl.connected = true
					} else {
						//stop already started caches since Agent cannot work
						//with a partially running system.
						agentCtrl.StopCaches()
					}
				}

			}
		}
	}
	return agentCtrl
}

//ConnectionTest checks the validity of the provided kubernetes configuration via
//kubeconfig file by trying to establish a connection to the API server.
func (ctrl *AgentController) ConnectionTest() bool {
	_, err := ctrl.kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: masterNodeLabel,
	})
	if err == nil {
		ctrl.valid = true
	}
	return ctrl.valid
}
