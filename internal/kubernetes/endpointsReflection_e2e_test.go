package kubernetes

import (
	"context"
	"fmt"
	"github.com/liqotech/liqo/api/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/internal/kubernetes/test"
	"github.com/liqotech/liqo/pkg/crdClient"
	"github.com/liqotech/liqo/pkg/virtualKubelet/apiReflection/apiReflection"
	"github.com/liqotech/liqo/pkg/virtualKubelet/translation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"testing"
	"time"
)

func TestHandleEpEventsNatting(t *testing.T) {
	// set the client in fake mode
	crdClient.Fake = true

	// create fake client for the home cluster
	hc, err := v1alpha1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	// create the fake client for the foreign cluster
	fc, err := v1alpha1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	// instantiate a fake provider
	p := KubernetesProvider{
		ReflectionManager:    &apiReflection.ReflectionManager{started: false},
		ntCache:              &namespaceNatting.namespaceNTCache{nattingTableName: test.ForeignClusterId},
		foreignPodCaches:     make(map[string]*podCache),
		homeEpCaches:         make(map[string]*epCache),
		foreignEpCaches:      make(map[string]*epCache),
		foreignClient:        fc,
		homeClient:           hc,
		startTime:            time.Time{},
		homeClusterID:        test.HomeClusterId,
		foreignClusterId:     test.ForeignClusterId,
		LocalRemappedPodCidr: test.LocalRemappedPodCIDR,
	}

	HandleEpEvents(t, p)
}

func TestHandleEpEventsNoNatting(t *testing.T) {
	// set the client in fake mode
	crdClient.Fake = true

	// create fake client for the home cluster
	hc, err := v1alpha1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	// create the fake client for the foreign cluster
	fc, err := v1alpha1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	// instantiate a fake provider
	p := KubernetesProvider{
		ReflectionManager: &apiReflection.ReflectionManager{started: false},
		ntCache:           &namespaceNatting.namespaceNTCache{nattingTableName: test.ForeignClusterId},
		foreignPodCaches:  make(map[string]*podCache),
		homeEpCaches:      make(map[string]*epCache),
		foreignEpCaches:   make(map[string]*epCache),
		foreignClient:     fc,
		homeClient:        hc,
		startTime:         time.Time{},
		homeClusterID:     test.HomeClusterId,
		foreignClusterId:  test.ForeignClusterId,
	}

	HandleEpEvents(t, p)

}

func HandleEpEvents(t *testing.T, p KubernetesProvider) {

	// start the fake cache for the namespaceNattingTable
	if err := p.startNattingCache(p.homeClient); err != nil {
		t.Fatal(err)
	}

	// create a new namespaceNattingTable and deploy it in the fake cache
	nt := test.CreateNamespaceNattingTable(p.foreignClusterId)
	if err := p.ntCache.Store.Add(nt); err != nil {
		t.Fatal(err)
	}

	// wait the namespace to be completely remotely reflected
	for {
		if p.isNamespaceReflected(test.Namespace) {
			break
		}
	}

	// ticker useful for make the test failing if some expected events are not triggered
	ticker := time.NewTicker(test.Timeout)
	done := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	// remote ep watcher is needed to be sure that all the expected home events are replicated in the
	// foreign cluster
	w, err := p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Watch(context.TODO(), metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		errChan <- err
		return
	}

	// This function checks that only the expected number of events is replicated remotely
	go func(errChan chan error, ticker *time.Ticker, w watch.Interface) {
		counter := 0
		for e := range w.ResultChan() {
			if e.Type == watch.Modified {
				t.Log("new update event on endpoints")
				counter++
			}

			if counter == test.EndpointsTestCases.ExpectedNumberOfEvents {
				close(done)
				ticker.Stop()
			}
			if counter > test.EndpointsTestCases.ExpectedNumberOfEvents {
				errChan <- fmt.Errorf("too many events occurred: %v events", counter)
				return
			}
		}
	}(errChan, ticker, w)

	// The home enpoints are updated
	if err := createEpEvents(p); err != nil {
		t.Fatal(err)
	}

loop:
	for {
		select {
		case <-done:
			break loop
		case <-ticker.C:
			t.Fatal("timeout")
		case err := <-errChan:
			t.Fatal(err)
		}
	}

	// delete the natting entry in the namespace natting table
	// this operation implies the reflection stop
	nt2 := nt.DeepCopy()
	nt2.Spec.NattingTable = nil
	nt2.Spec.DeNattingTable = nil
	if err = p.ntCache.Store.Update(nt2); err != nil {
		t.Fatal(err)
	}

	// Wait the namespace reflection to be stopped
	for {
		if !p.isNamespaceReflected(test.Namespace) {
			break
		}
	}

	// get the foreign endpoints
	ep, err := p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Get(context.TODO(), test.EndpointsName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// assert that the home endpoints have been correctly updated in the remote cluster
	if !assertEndpointsCoherency(p.LocalRemappedPodCidr, ep.Subsets, test.EndpointsTestCases.ExpectedEndpoints.Subsets) {
		t.Fatal("the received ep doesn't match with the expected one")
	}
	t.Log("the received ep matches with the expected one")

	// last check to be sure that only the expected number of foreign events has been triggered
	select {
	case err = <-errChan:
		t.Fatal(err)
	default:
		w.Stop()
	}
}

func createEpEvents(p KubernetesProvider) error {
	// create a new endpoints object in the home cluster
	ep := test.EndpointsTestCases.InputEndpoints
	_, err := p.homeClient.Client().CoreV1().Endpoints(test.Namespace).Create(context.TODO(), &ep, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// create a new endpoints object in the foreign cluster
	_, err = p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Create(context.TODO(), &ep, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	for _, s := range test.EndpointsTestCases.InputSubsets {
		ep.Subsets = s
		_, err = p.homeClient.Client().CoreV1().Endpoints(test.Namespace).Update(context.TODO(), &ep, metav1.UpdateOptions{})
		time.Sleep(time.Millisecond)
		if err != nil {
			return err
		}
	}

	return nil
}

func assertEndpointsCoherency(podCIDR string, received, expected []corev1.EndpointSubset) bool {
	if len(received) != len(expected) {
		return false
	}
	for i := 0; i < len(received); i++ {
		if len(received[i].Addresses) != len(expected[i].Addresses) {
			return false
		}
		for j := 0; j < len(received[i].Addresses); j++ {
			addr := translation.ChangePodIp(podCIDR, expected[i].Addresses[j].IP)
			if received[i].Addresses[j].IP != addr {
				return false
			}
		}
	}
	return true
}
