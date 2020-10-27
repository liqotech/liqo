package client

import (
	"fmt"
	testutil "github.com/liqotech/liqo/internal/virtualKubelet/test/util"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"os"
	"strconv"
	"testing"
)

//configLocal params
var nodePort = 32000
var masterNodeIP = "10.0.0.1"
var getLiqoDashServiceReactor ktesting.ReactionFunc = func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	getAction := action.(ktesting.GetAction)
	servName := getAction.GetName()
	servNamespace := getAction.GetNamespace()
	dashConf := GetAgentController().agentConf.dashboard
	if servName != dashConf.service || servNamespace != dashConf.namespace {
		return false, nil, nil
	}
	liqoServ := testutil.FakeService(dashConf.namespace, dashConf.service,
		"10.0.0.2", "TCP", 80)
	liqoServ.Spec.Type = corev1.ServiceTypeNodePort
	liqoServ.Spec.Ports[0].Name = "https"
	liqoServ.Spec.Ports[0].NodePort = int32(nodePort)
	return true, liqoServ, nil
}
var listMasterNodeReactor ktesting.ReactionFunc = func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	labelsMap := make(map[string]string)
	labelsMap[masterNodeLabel] = ""
	var labelSet = labels.Set{}
	labelSet[masterNodeLabel] = ""
	listAction := action.(ktesting.ListAction)
	if !listAction.GetListRestrictions().Labels.Matches(labelSet) {
		return false, nil, nil
	}
	nodeL := corev1.NodeList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: strconv.Itoa(1),
		},
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-master_node",
					ResourceVersion: strconv.Itoa(1),
					Labels:          labelsMap,
				},
				Status: corev1.NodeStatus{
					Phase: corev1.NodeRunning,
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: masterNodeIP,
						},
					},
				},
			},
		},
	}
	return true, &nodeL, nil
}

//configRemote params
var ingressTestHost = "test.host.net"
var listLiqoDashIngressesReactor ktesting.ReactionFunc = func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	listAction := action.(ktesting.ListAction)
	dashConf := GetAgentController().agentConf.dashboard
	labelSet := labels.Set{}
	labelSet["app"] = dashConf.label
	if !listAction.GetListRestrictions().Labels.Matches(labelSet) || listAction.GetNamespace() != dashConf.namespace {
		return false, nil, nil
	}
	labelsMap := make(map[string]string)
	labelsMap["app"] = dashConf.label
	ingressList := v1beta1.IngressList{
		ListMeta: metav1.ListMeta{},
		Items: []v1beta1.Ingress{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       dashConf.namespace,
					ResourceVersion: strconv.Itoa(1),
					Labels:          labelsMap,
				},
				Spec: v1beta1.IngressSpec{
					TLS: []v1beta1.IngressTLS{
						{
							Hosts: []string{
								ingressTestHost,
							},
							SecretName: "",
						},
					},
				},
			},
		},
	}
	return true, &ingressList, nil
}

//getLiqoDashSecret params
var testData = "test-data"
var getLiqoDashServiceAccountReactor ktesting.ReactionFunc = func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	getAction := action.(ktesting.GetAction)
	dashConf := GetAgentController().agentConf.dashboard
	secrTestName := dashConf.serviceAccount + "-token-test"
	if getAction.GetName() != dashConf.serviceAccount || getAction.GetNamespace() != dashConf.namespace {
		return false, nil, nil
	}
	servAcc := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dashConf.serviceAccount,
			Namespace:       dashConf.namespace,
			ResourceVersion: strconv.Itoa(1),
		},
		Secrets: []corev1.ObjectReference{
			{
				Namespace:       dashConf.namespace,
				Name:            secrTestName,
				ResourceVersion: strconv.Itoa(1),
			},
		},
	}
	return true, &servAcc, nil
}
var getLiqoDashSecretReactor ktesting.ReactionFunc = func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	getAction := action.(ktesting.GetAction)
	dashConf := GetAgentController().agentConf.dashboard
	secrTestName := dashConf.serviceAccount + "-token-test"
	if getAction.GetName() != secrTestName || getAction.GetNamespace() != dashConf.namespace {
		return false, nil, nil
	}
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            secrTestName,
			Namespace:       dashConf.namespace,
			ResourceVersion: strconv.Itoa(1),
		},
		Data: make(map[string][]byte),
	}
	secret.Data["token"] = []byte(testData)
	return true, &secret, nil
}

//acquireDashboardConfig params
var listLiqoDashPodsReactor ktesting.ReactionFunc = func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
	dashConf := GetAgentController().agentConf.dashboard
	labelSet := labels.Set{}
	labelSet["app"] = dashConf.label
	listAction := action.(ktesting.ListAction)
	if !listAction.GetListRestrictions().Labels.Matches(labelSet) || listAction.GetNamespace() != dashConf.namespace {
		return false, nil, nil
	}
	labelsMap := make(map[string]string)
	labelsMap["app"] = dashConf.label
	podList := corev1.PodList{
		ListMeta: metav1.ListMeta{
			ResourceVersion: strconv.Itoa(1),
		},
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: dashConf.namespace,
					Labels:    labelsMap,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}
	return true, &podList, nil
}

func TestAgentController_AcquireDashboardConfig(t *testing.T) {
	UseMockedAgentController()
	DestroyMockedAgentController()
	ctrl := GetAgentController()
	//test no LiqoDash pod present and running
	err := ctrl.AcquireDashboardConfig()
	assert.Error(t, err, "AcquireDashboardConfig should return error if no \nLiqoDash Pod "+
		"is 'Running' ")
	//test function return with Local configuration
	fakeKubeClient := ctrl.kubeClient.(*fake.Clientset)
	fakeKubeClient.Fake.PrependReactor("list", "pods", listLiqoDashPodsReactor)
	fakeKubeClient.Fake.PrependReactor("list", "nodes", listMasterNodeReactor)
	fakeKubeClient.Fake.PrependReactor("get",
		"services", getLiqoDashServiceReactor)
	err = ctrl.AcquireDashboardConfig()
	assert.NoError(t, err, "AcquireDashboardConfig should return nil when\n"+
		"a LiqoDash pod is running and at least one config is available")
	_, isSet := os.LookupEnv(EnvLiqoDashHost)
	assert.True(t, isSet, "EnvLiqoDashHost should be set")
	_, isSet = os.LookupEnv(EnvLiqoDashPort)
	assert.True(t, isSet, "EnvLiqoDashPort should be set")
}

func TestAgentController_getDashboardConfigLocal(t *testing.T) {
	UseMockedAgentController()
	DestroyMockedAgentController()
	var err error
	if err = os.Unsetenv(EnvLiqoDashHost); err != nil {
		t.Fatal("unable to unset ENV liqodash HOST")
	}
	if err = os.Unsetenv(EnvLiqoDashPort); err != nil {
		t.Fatal("unable to unset ENV liqodash PORT")
	}
	ctrl := GetAgentController()
	fakeKubeClient := ctrl.kubeClient.(*fake.Clientset)
	fakeKubeClient.Fake.PrependReactor("list", "nodes", listMasterNodeReactor)
	fakeKubeClient.Fake.PrependReactor("get",
		"services", getLiqoDashServiceReactor)
	res := ctrl.getDashboardConfigLocal()
	assert.True(t, res, "DashboardConfigLocal should return true")
	var dashHost, dashPort string
	var envSet bool
	if dashHost, envSet = os.LookupEnv(EnvLiqoDashHost); envSet {
		assert.Equal(t, fmt.Sprintf("https://%s", masterNodeIP), dashHost)
	} else {
		t.Fatal("ENV liqodash HOST not set after configLocal")
	}
	if dashPort, envSet = os.LookupEnv(EnvLiqoDashPort); envSet {
		assert.Equal(t, fmt.Sprint(nodePort), dashPort)
	} else {
		t.Fatal("ENV liqodash PORT not set after configLocal")
	}
}

func TestAgentController_getDashboardConfigRemote(t *testing.T) {
	UseMockedAgentController()
	DestroyMockedAgentController()
	var err error
	if err = os.Unsetenv(EnvLiqoDashHost); err != nil {
		t.Fatal("unable to unset ENV liqodash HOST")
	}
	if err = os.Unsetenv(EnvLiqoDashPort); err != nil {
		t.Fatal("unable to unset ENV liqodash PORT")
	}
	ctrl := GetAgentController()
	fakeKubeClient := ctrl.kubeClient.(*fake.Clientset)
	fakeKubeClient.Fake.PrependReactor("list", "ingresses", listLiqoDashIngressesReactor)
	res := ctrl.getDashboardConfigRemote()
	assert.True(t, res, "DashboardConfigRemote should return true")
	var dashHost, dashPort string
	var envSet bool
	if dashHost, envSet = os.LookupEnv(EnvLiqoDashHost); envSet {
		assert.Equal(t, fmt.Sprintf("https://%s", ingressTestHost), dashHost)
	} else {
		t.Fatal("ENV liqodash HOST not set after configRemote")
	}
	if dashPort, envSet = os.LookupEnv(EnvLiqoDashPort); envSet {
		assert.Equal(t, "", dashPort)
	} else {
		t.Fatal("ENV liqodash PORT not set after configRemote")
	}
}

func TestAgentController_GetLiqoDashSecret(t *testing.T) {
	UseMockedAgentController()
	DestroyMockedAgentController()
	ctrl := GetAgentController()
	fakeKubeClient := ctrl.kubeClient.(*fake.Clientset)
	fakeKubeClient.Fake.PrependReactor("get", "secrets", getLiqoDashSecretReactor)
	fakeKubeClient.Fake.PrependReactor("get", "serviceaccounts", getLiqoDashServiceAccountReactor)
	token, err := ctrl.GetLiqoDashSecret()
	if err != nil {
		t.Fatal("GetLiqoDashSecret should not return an error")
	}
	assert.Equal(t, testData, *token, "secret token differs from the test one")

}
