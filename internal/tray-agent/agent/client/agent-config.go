package client

import (
	"errors"
	clusterConfig "github.com/liqotech/liqo/apis/config/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
)

//agentConfiguration contains Agent config parameters.
type agentConfiguration struct {
	//valid specifies whether the agentConfiguration has been correctly initialized.
	valid bool
	//dashboard contains parameters for LiqoDash.
	dashboard *dashConfig
}

//dashConfig contains the parameters required for Agent
//to access LiqoDash.
type dashConfig struct {
	//namespace of LiqoDash.
	namespace string
	//service name of LiqoDash.
	service string
	//serviceAccount name of LiqoDash.
	serviceAccount string
	//ingress name of LiqoDash.
	ingress string
	//label is the value for the "app" label, which is
	//used by all LiqoDash related resources.
	label string
}

//acquireClusterConfiguration initializes the AgentController configuration
//by retrieving data from the ClusterConfig CR.
func (ctrl *AgentController) acquireClusterConfiguration() {
	if !ctrl.connected {
		return
	}
	aConf := ctrl.agentConf
	var clConf *clusterConfig.ClusterConfig
	var err error
	if ctrl.Mocked() {
		clConf, err = createClusterConfig()
	} else {
		clConf, err = ctrl.getConfig()
	}
	if err != nil {
		return
	}
	agentConfig := clConf.Spec.AgentConfig
	aConf.dashboard = &dashConfig{
		namespace:      agentConfig.DashboardConfig.Namespace,
		service:        agentConfig.DashboardConfig.Service,
		serviceAccount: agentConfig.DashboardConfig.ServiceAccount,
		label:          agentConfig.DashboardConfig.AppLabel,
		ingress:        agentConfig.DashboardConfig.Ingress,
	}
	aConf.valid = true
}

//getConfig retrieves the ClusterConfig CR which contains configuration data.
func (ctrl *AgentController) getConfig() (*clusterConfig.ClusterConfig, error) {
	if !ctrl.connected {
		return nil, errors.New("no connection available")
	}
	objL, err := ctrl.Controller(CRClusterConfig).Resource(string(CRClusterConfig)).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	confL := objL.(*clusterConfig.ClusterConfigList)
	if len(confL.Items) < 1 {
		return nil, errors.New("no ClusterConfig is present")
	}
	return &(confL.Items[0]), nil
}

//createClusterConfig creates a mocked ClusterConfig CR.
func createClusterConfig() (*clusterConfig.ClusterConfig, error) {
	confObj := &clusterConfig.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "testConfig",
			ResourceVersion: strconv.Itoa(1),
		},
		Spec: clusterConfig.ClusterConfigSpec{
			AgentConfig: clusterConfig.AgentConfig{
				DashboardConfig: clusterConfig.DashboardConfig{
					Namespace:      "liqo",
					Service:        "liqodashService",
					ServiceAccount: "liqodashSA",
					AppLabel:       "liqodash",
					Ingress:        "liqo-dashboard-ingress",
				},
			},
		},
	}
	return confObj, nil
}

//ValidConfiguration returns whether AgentController configuration data
//have been correctly initialized.
func (ctrl *AgentController) ValidConfiguration() bool {
	return ctrl.agentConf.valid
}
