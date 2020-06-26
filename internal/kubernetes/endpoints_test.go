package kubernetes

import (
	"github.com/liqoTech/liqo/api/namespaceNattingTable/v1"
	"github.com/liqoTech/liqo/internal/kubernetes/test"
	"github.com/liqoTech/liqo/pkg/crdClient/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
)

func TestManageEpEvent(t *testing.T) {
	v1alpha1.Fake = true

	homeClient, err := v1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	foreignClient, err := v1.CreateClient("")
	if err != nil {
		t.Fatal(err)
	}

	p := KubernetesProvider{
		Reflector:        &Reflector{started: false},
		ntCache:          &namespaceNTCache{nattingTableName: test.ForeignClusterId},
		foreignClient:    foreignClient,
		homeClient:       homeClient,
		startTime:        time.Time{},
		foreignClusterId: test.ForeignClusterId,
		homeClusterID:    test.HomeClusterId,
	}

	if err := p.startNattingCache(homeClient); err != nil {
		t.Fatal(err)
	}

	nt := createNamespaceNattingTable()

	if err = p.ntCache.Store.Add(nt); err != nil {
		t.Fatal(err)
	}

	for {
		if p.isNamespaceReflected(test.Namespace) {
			break
		}
	}

	if err := createEpEvents(p); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{}, 1)
	errChan := make(chan error, 1)
	go func(errChan chan error) {
		w, err := p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Watch(metav1.ListOptions{
			Watch: true,
		})
		if err != nil {
			errChan <- err
			return
		}

		i := 0
		for range w.ResultChan() {
			i++
			if i == test.EndpointsTestCases.ExpectedNumberOfEvents {
				break
			}
		}
		w.Stop()
		close(done)
	}(errChan)

loop:
	for {
		select {
		case <-done:
			break loop
		case err := <-errChan:
			t.Fatal(err)
		}
	}

	ep, err := p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Get(test.EndpointsName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	nt2 := nt.DeepCopy()
	nt2.Spec.NattingTable = nil
	nt2.Spec.DeNattingTable = nil
	if err = p.ntCache.Store.Update(nt2); err != nil {
		t.Fatal(err)
	}

	for {
		if !p.isNamespaceReflected(test.Namespace) {
			break
		}
	}

	if !test.AssertEndpointsCorrectness(ep.Subsets, test.EndpointsTestCases.ExpectedEndpoints.Subsets) {
		t.Fatal("the received ep doesn't match with the expected one")
	}
}

func createEpEvents(p KubernetesProvider) error {
	// create a new endpoints object in the foreign cluster
	ep := test.EndpointsTestCases.InputEndpoints
	_, err := p.homeClient.Client().CoreV1().Endpoints(test.Namespace).Create(ep)
	if err != nil {
		return err
	}

	// create a new endpoints object in the home cluster
	_, err = p.foreignClient.Client().CoreV1().Endpoints(test.NattedNamespace).Create(ep)
	if err != nil {
		return err
	}

	for _, s := range test.EndpointsTestCases.InputSubsets {
		ep.Subsets = s
		_, err = p.homeClient.Client().CoreV1().Endpoints(test.Namespace).Update(ep)
		if err != nil {
			return err
		}
	}

	return nil
}

func createNamespaceNattingTable() *v1.NamespaceNattingTable {
	return &v1.NamespaceNattingTable{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: test.ForeignClusterId,
		},
		Spec: v1.NamespaceNattingTableSpec{
			ClusterId: test.ForeignClusterId,
			NattingTable: map[string]string{
				test.Namespace: test.NattedNamespace,
			},
			DeNattingTable: map[string]string{
				test.NattedNamespace: test.Namespace,
			},
		},
	}
}
