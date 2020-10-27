package client

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"os"
	"strings"
)

const (
	//masterNodeLabel is the label name associated with master nodes.
	masterNodeLabel = "node-role.kubernetes.io/master"
	//EnvLiqoDashHost defines the env var for the HOST part of the LiqoDash address.
	EnvLiqoDashHost = "LIQODASH_HOST"
	//EnvLiqoDashPort defines the env var for the PORT part of the LiqoDash address.
	EnvLiqoDashPort = "LIQODASH_PORT"
)

//AcquireDashboardConfig tries to retrieve required data to access the LiqoDash service.
//
//If a valid configuration is found, the EnvLiqoDashHost and EnvLiqoDashPort env vars are set.
func (ctrl *AgentController) AcquireDashboardConfig() error {
	//cleanup LiqoDash env vars
	if !ctrl.Connected() || !ctrl.ValidConfiguration() {
		return errors.New("cluster connection not available")
	}
	var err error
	if err = os.Unsetenv(EnvLiqoDashHost); err != nil {
		return err
	}
	if err = os.Unsetenv(EnvLiqoDashPort); err != nil {
		return err
	}
	//preliminary check to verify the LiqoDash pod is running
	var dashPodL *corev1.PodList
	dashConf := ctrl.agentConf.dashboard
	dashPodL, err = ctrl.kubeClient.CoreV1().Pods(dashConf.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + dashConf.label,
		FieldSelector: fields.OneTermEqualSelector("status.phase", "Running").String(),
	})
	if err != nil {
		return errors.New("cluster connection not available")
	}
	if len(dashPodL.Items) < 1 {
		return errors.New("the LiqoDash is not currently available")
	}
	//check if all LiqoDash pods are ready to serve
	ready := true
loop:
	for _, pod := range dashPodL.Items {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
				ready = false
				break loop
			}
		}
	}
	if !ready {
		return errors.New("the LiqoDash is not currently available")
	}
	/*-----------------------------------------------------------------------------------
	CASE 1: check the presence of an ingress for the LiqoDash
	-------------------------------------------------------------------------------------*/
	var ok bool
	if ok = ctrl.getDashboardConfigRemote(); !ok {
		/*-----------------------------------------------------------------------------------
		CASE 2: check the presence of a Service NodePort for the LiqoDash
		-------------------------------------------------------------------------------------*/
		if ok = ctrl.getDashboardConfigLocal(); !ok {
			return errors.New("cannot establish a connection to LiqoDash")
		}
	}
	return nil
}

//getDashboardConfigRemote searches for a valid configuration required to
//remotely connect with the LiqoDash.
//
//In case of success, it sets the env vars specified by EnvLiqoDashHost
//and EnvLiqoDashPort with proper values.
func (ctrl *AgentController) getDashboardConfigRemote() bool {
	if !ctrl.Connected() || !ctrl.ValidConfiguration() {
		return false
	}
	/*search for a LiqoDash Ingress. To increase security, it must contain
	a 'tls' field with at least one explicitly specified 'host' (https connection)*/
	dashConf := ctrl.agentConf.dashboard
	ingrL, err := ctrl.kubeClient.NetworkingV1beta1().Ingresses(dashConf.namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + dashConf.label,
	})
	if err != nil || len(ingrL.Items) < 1 {
		return false
	}
	ingress := ingrL.Items[0]
	if len(ingress.Spec.TLS) > 0 {
		hosts := ingress.Spec.TLS[0].Hosts
		if len(hosts) > 0 {
			dashHost := hosts[0]
			if err = os.Setenv(EnvLiqoDashHost, fmt.Sprintf(
				"https://%s", dashHost)); err == nil {
				if err = os.Setenv(EnvLiqoDashPort, ""); err == nil {
					return true
				}
			}
		}
	}
	return false
}

//getDashboardConfigLocal searches for a valid configuration required to
//establish a local connection to the LiqoDash.
//
//In case of success, it sets the env vars specified by EnvLiqoDashHost
//and EnvLiqoDashPort with proper values.
func (ctrl *AgentController) getDashboardConfigLocal() bool {
	if !ctrl.Connected() || !ctrl.ValidConfiguration() {
		return false
	}
	c := ctrl.kubeClient
	dashConf := ctrl.agentConf.dashboard
	var nodePortNo, masterIP string
	found := false
	/*search for a LiqoDash Service of type NodePort*/
	service, err := c.CoreV1().Services(dashConf.namespace).Get(context.TODO(), dashConf.service, metav1.GetOptions{})
	if err == nil && service.Spec.Type == corev1.ServiceTypeNodePort {
		ports := service.Spec.Ports
		for i := range ports {
			port := ports[i]
			if port.Name == "https" {
				nodePortNo = fmt.Sprint(port.NodePort)
				found = true
				break
			}
		}
	}
	/*A valid port has been found.
	For the local connection, the master node IP address will be used.*/
	if found {
		found = false
		nodeL, err := c.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
			LabelSelector: masterNodeLabel,
		})
		if err == nil && len(nodeL.Items) > 0 {
			for _, addr := range nodeL.Items[0].Status.Addresses {
				if addr.Type == corev1.NodeInternalIP {
					masterIP = addr.Address
					found = true
					break
				}
			}
		}
	}
	if found {
		/*having found both address and port, it is possible to set
		the two env vars*/
		if err = os.Setenv(EnvLiqoDashHost, fmt.Sprintf(
			"https://%s", masterIP)); err == nil {
			if err = os.Setenv(EnvLiqoDashPort, nodePortNo); err == nil {
				return true
			}
		}
	}
	return false
}

//GetLiqoDashSecret returns the access token for the LiqoDash service.
func (ctrl *AgentController) GetLiqoDashSecret() (*string, error) {
	var token = ""
	if !ctrl.Connected() || !ctrl.ValidConfiguration() {
		return &token, errors.New("no connection to the cluster")
	}
	errNoToken := errors.New("cannot retrieve token")
	/*In order to better prune its search, the secret is retrieved by its name, using the
	service account associated with it.*/
	c := ctrl.kubeClient
	dashConf := ctrl.agentConf.dashboard
	liqoSA, err := c.CoreV1().ServiceAccounts(dashConf.namespace).Get(context.TODO(), dashConf.serviceAccount, metav1.GetOptions{})
	if err != nil {
		return &token, errNoToken
	}
	found := false
	var secretName string
	tokenPrefixName := dashConf.serviceAccount + "-token"
	for i := range liqoSA.Secrets {
		secret := liqoSA.Secrets[i]
		if strings.HasPrefix(secret.Name, tokenPrefixName) {
			found = true
			secretName = secret.Name
			break
		}
	}
	if found {
		if secret, err := c.CoreV1().Secrets(dashConf.namespace).Get(context.TODO(), secretName, metav1.GetOptions{}); err == nil {
			token = fmt.Sprintf("%s", secret.Data["token"])
			return &token, nil
		}
	}
	return &token, errNoToken
}
