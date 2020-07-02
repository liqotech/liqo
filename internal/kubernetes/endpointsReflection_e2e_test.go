package kubernetes

import (
	"fmt"
	"github.com/liqoTech/liqo/api/namespaceNattingTable/v1"
	"github.com/liqoTech/liqo/internal/kubernetes/test"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"testing"
	"time"
)

func TestHandleEpEvents(t *testing.T) {
	// set the client in fake mode
	v1alpha1.Fake = true

	// create fake client for the home cluster
	homeClient, err := v1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	// create the fake client for the foreign cluster
	foreignClient, err := v1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	// instantiate a fake provider
	p := KubernetesProvider{
		Reflector:        &Reflector{started: false},
		ntCache:          &namespaceNTCache{nattingTableName: test.ForeignClusterId},
		foreignClient:    foreignClient,
		homeClient:       homeClient,
		startTime:        time.Time{},
		foreignClusterId: test.ForeignClusterId,
		homeClusterID:    test.HomeClusterId,
	}

	// start the fake cache for the namespaceNattingTable
	if err := p.startNattingCache(homeClient); err != nil {
		t.Fatal(err)
	}

	// create a new namespaceNattingTable and deploy it in the fake cache
	nt := test.CreateNamespaceNattingTable()
	if err = p.ntCache.Store.Add(nt); err != nil {
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
	w, err := p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Watch(metav1.ListOptions{
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
	ep, err := p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Get(test.EndpointsName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// assert that the home endpoints have been correctly updated in the remote cluster
	if !test.AssertEndpointsCoherency(ep.Subsets, test.EndpointsTestCases.ExpectedEndpoints.Subsets) {
		t.Fatal("the received ep doesn't match with the expected one")
	}

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
	_, err := p.homeClient.Client().CoreV1().Endpoints(test.Namespace).Create(ep)
	if err != nil {
		return err
	}

	// create a new endpoints object in the foreign cluster
	_, err = p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Create(ep)
	if err != nil {
		return err
	}

	for _, s := range test.EndpointsTestCases.InputSubsets {
		ep.Subsets = s
		_, err = p.homeClient.Client().CoreV1().Endpoints(test.Namespace).Update(ep)
		time.Sleep(time.Millisecond)
		if err != nil {
			return err
		}
	}

	return nil
}
