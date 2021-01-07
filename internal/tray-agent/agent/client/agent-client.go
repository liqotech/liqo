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
	//EnvLiqoPath defines the env var containing the path of the root directory of the Liqo Agent on the local file system.
	EnvLiqoPath = "LIQO_PATH"
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

//NotifyChan is the wrapper type for generic data sent over a NotifyChannel. After receiving such element from a
//chan, it is then possible to try its conversion into a specific type.
type NotifyDataGeneric interface{}

//AgentController is the data structure that manages Tray Agent interaction with the cluster.
type AgentController struct {
	//notifyChannels is a set of channels used by the cache logic to notify a watched event.
	notifyChannels map[NotifyChannel]chan NotifyDataGeneric
	//kubeClient is a standard kubernetes client.
	kubeClient kubernetes.Interface
	//agentConf contains Liqo Agent configuration parameters acquired from the cluster.
	agentConf *agentConfiguration
	//crdManager manages CRD operations.
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
func (ctrl *AgentController) NotifyChannel(channelType NotifyChannel) chan NotifyDataGeneric {
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

/*acquireKubeconfig sets the EnvLiqoKConfig env variable.
EnvLiqoKConfig represents the path of a kubeconfig file required to let
the client connect to the local cluster.

- At first, the function uses the file path of the 'kubeconf' program argument.

- If that argument is not provided, it checks a valid kubeconfig path in the LocalConfig acquired from the config file.

- If none of the two options are available, it defaults to $HOME/.kube/config.

- If the selected path does not point to an existing file, users are asked to manually select a valid one.

- At the end of the process, the env var EnvLiqoKConfig is set only if an existing file has been indicated.
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
		//CASE 1: use a command line parameter
		kubePath := filepath.Join(os.Getenv("HOME"), ".kube")
		defaultKubePath := filepath.Join(kubePath, "config")
		flagOnce.Do(func() {
			kubeconfArg = flag.String("kubeconf", defaultKubePath,
				"[OPT] absolute path to the kubeconfig file."+
					" Default = $HOME/.kube/config")
			flag.Parse()
		})
		//CASE 2: no explicit parameter: check if a kubeconfig path has been indicated in a config file
		if *kubeconfArg == defaultKubePath {
			if conf, valid := GetLocalConfig(); valid {
				if kubeconf := conf.GetKubeconfig(); kubeconf != "" {
					*kubeconfArg = kubeconf
				}
			}
		}
		//CASE 3: use default value
		//check if selected path actually match a file
		if _, err := os.Stat(*kubeconfArg); os.IsNotExist(err) {
			//CASE 4: ask manual file selection
			ok, _ := dlgs.Question("NO VALID KUBECONFIG FILE FOUND",
				"Liqo could not find a valid kubeconfig file.\n "+
					"Do you want to select one?", false)
			if ok {
				filePath, selected, _ := dlgs.File("Select file", "", false)
				if selected {
					path = filePath
					found = true
					//save new preferred choice to config file
					//if there exist a valid configuration, update it. Otherwise create a new one.
					config, valid := GetLocalConfig()
					if !valid {
						config = NewLocalConfig()
						config.Valid = true
					}
					config.SetKubeconfig(filePath)
					if err := SaveLocalConfig(); err != nil {
						_, _ = dlgs.Warning("LIQO AGENT", "Liqo Agent could not save settings changes")
					}
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
		agentCtrl = &AgentController{
			agentConf: &agentConfiguration{},
		}
		agentCtrl.mocked = mockedController
		//init the notifyChannels that are kept open during the entire Agent execution.
		agentCtrl.notifyChannels = make(map[NotifyChannel]chan NotifyDataGeneric)
		for _, i := range notifyChannelNames {
			agentCtrl.notifyChannels[i] = make(chan NotifyDataGeneric, notifyBuffLength)
		}
		var err error
		//acquire configuration, try to connect clients, start caches.
		acquireKubeconfig()
		if agentCtrl.kubeClient, err = createKubeClient(); err == nil {
			if err = agentCtrl.initCRDManager(); err == nil {
				if agentCtrl.ConnectionTest() {
					if err = agentCtrl.StartCaches(); err == nil {
						agentCtrl.connected = true
						//init configuration data
						agentCtrl.acquireClusterConfiguration()
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
