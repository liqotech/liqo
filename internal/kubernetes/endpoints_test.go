package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
	"testing"
	"time"
)

const (
	testNamespace = "test"
)

var endpoints = &corev1.Endpoints{
	ObjectMeta: metav1.ObjectMeta{
		Name:                       "service1",
		Namespace:                  testNamespace,
	},
	Subsets:    []corev1.EndpointSubset{
		{
			Addresses:         []corev1.EndpointAddress{
				{
					IP:        "10.0.0.1",
					Hostname:  "host1",
					NodeName:  nil,
					TargetRef: nil,
				},
			},
			NotReadyAddresses: nil,
			Ports: []corev1.EndpointPort{
				{
					Name: "port",
					Port: 8000,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	},
}


func TestManageEpEvent(t *testing.T) {

	client := fake.NewSimpleClientset()
	stop := make(chan struct{}, 1)
	errChan := make(chan error, 1000)

	go createEpEvents(client, stop, errChan)
	
	result, err := client.CoreV1().Endpoints(testNamespace).Watch(metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		t.Fatal(err)
	}

loop:
	for {
		select {
		case err = <- errChan:
			t.Fatal(err)
		case <- stop:
			klog.Info("client populated")
			break loop
		case event := <- result.ResultChan():
			ep := event.Object.(*corev1.Endpoints)
			klog.Infof("new event of type %v on endpoint %v", event.Type, ep.Name)
		default:
			break
		}
	}
}

func createEpEvents(client *fake.Clientset, stop chan struct{}, errChan chan error) {
	// create a new endpoints object
	_, err := client.CoreV1().Endpoints(testNamespace).Create(endpoints)
	if err != nil {
		errChan <- err
		return
	}

	// update the endpoint object
	addr := corev1.EndpointAddress{
		IP:        "10.0.0.2",
		Hostname:  "host2",
		NodeName:  nil,
		TargetRef: nil,
	}
	endpoints.Subsets[0].Addresses = append(endpoints.Subsets[0].Addresses, addr)
	_, err = client.CoreV1().Endpoints(testNamespace).Update(endpoints)
	if err != nil {
		errChan <- err
		return
	}

	// this sleep has to be removed in some way (it forces the consumer to be synchronized with the producer
	time.Sleep(1*time.Second)
	close(stop)
}


