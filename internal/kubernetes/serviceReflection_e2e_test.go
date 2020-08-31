package kubernetes

import (
	"context"
	"errors"
	"fmt"
	v1 "github.com/liqoTech/liqo/api/virtualKubelet/v1alpha1"
	"github.com/liqoTech/liqo/internal/kubernetes/test"
	"github.com/liqoTech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"testing"
	"time"
)

func TestHandleServiceEvents(t *testing.T) {
	// set the client in fake mode
	crdClient.Fake = true

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
	p := &KubernetesProvider{
		Reflector:        &Reflector{started: false},
		ntCache:          &namespaceNTCache{nattingTableName: test.ForeignClusterId},
		foreignPodCaches: make(map[string]*podCache),
		homeEpCaches:     make(map[string]*epCache),
		foreignEpCaches:  make(map[string]*epCache),
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
	createsdDone := make(chan struct{}, 1)
	updatesDone := make(chan struct{}, 1)
	deletesDone := make(chan struct{}, 1)
	errChan := make(chan error, 1)

	// remote ep watcher is needed to be sure that all the expected home events are replicated in the
	// foreign cluster
	w, err := p.foreignClient.Client().CoreV1().Services(test.NattedNamespace).Watch(context.TODO(), metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		errChan <- err
		return
	}

	go serviceEventsMonitoring(errChan, createsdDone, updatesDone, deletesDone, ticker, w)
	go svcCreation(p, errChan)

loop:
	for {
		select {
		case <-createsdDone:
			if err := verifySvcConsistency(p, "creation"); err != nil {
				t.Fatal(err)
			}
			go svcUpdate(p, errChan)
		case <-updatesDone:
			if err = verifySvcConsistency(p, "update"); err != nil {
				t.Fatal(err)
			}
			go svcDelete(p, errChan)
		case <-deletesDone:
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

	if err := verifySvcConsistency(p, "delete"); err != nil {
		t.Fatal(err)
	}

	// last check to be sure that only the expected number of foreign events has been triggered
	select {
	case err = <-errChan:
		t.Fatal(err)
	default:
		w.Stop()
	}
}

func serviceEventsMonitoring(errChan chan error, createsDone, updatesDone, deletesDone chan struct{}, ticker *time.Ticker, w watch.Interface) {
	// counters for event type
	creates := 0
	updates := 0
	deletes := 0

	// needed to avoid close of closed channel
	var cc, uc, dc bool

	for e := range w.ResultChan() {
		klog.Infof("TEST - New foreign event of type %v", e.Type)
		switch e.Type {
		case watch.Added:
			creates++
		case watch.Modified:
			updates++
		case watch.Deleted:
			deletes++
		default:
			errChan <- fmt.Errorf("unexpected event of type %v", e.Type)
			return
		}

		if creates == len(test.ServiceTestCases.InputServices) && !cc {
			createsDone <- struct{}{}
			cc = true
		}
		if updates == len(test.ServiceTestCases.UpdateServices) && !uc {
			updatesDone <- struct{}{}
			uc = true
		}
		if deletes == len(test.ServiceTestCases.DeleteServices) && !dc {
			close(deletesDone)
			dc = true
			ticker.Stop()
		}

		if creates > len(test.ServiceTestCases.InputServices) {
			errChan <- errors.New("too many create events")
			return
		}
		if updates > len(test.ServiceTestCases.UpdateServices) {
			errChan <- errors.New("too many update events")
			return
		}
		if deletes > len(test.ServiceTestCases.DeleteServices) {
			errChan <- errors.New("too many delete events")
			return
		}
	}
}

func svcCreation(p *KubernetesProvider, chanError chan error) {
	klog.Info("TEST - starting svc creation")
	for _, s := range test.ServiceTestCases.InputServices {
		_, err := p.homeClient.Client().CoreV1().Services(test.Namespace).Create(context.TODO(), s, metav1.CreateOptions{})
		if err != nil {
			chanError <- err
			return
		}
	}
}

func svcUpdate(p *KubernetesProvider, chanError chan error) {
	klog.Info("TEST - starting svc update")
	for _, s := range test.ServiceTestCases.UpdateServices {
		_, err := p.homeClient.Client().CoreV1().Services(test.Namespace).Update(context.TODO(), s, metav1.UpdateOptions{})
		if err != nil {
			chanError <- err
			return
		}
	}
}

func svcDelete(p *KubernetesProvider, chanError chan error) {
	klog.Info("TEST - starting svc delete")
	for _, s := range test.ServiceTestCases.DeleteServices {
		err := p.homeClient.Client().CoreV1().Services(test.Namespace).Delete(context.TODO(), s.Name, metav1.DeleteOptions{})
		if err != nil {
			chanError <- err
			return
		}
	}
}

func verifySvcConsistency(p *KubernetesProvider, event string) error {
	klog.Infof("TEST - Asserting status coherency after %v", event)
	homeSvcs, err := p.homeClient.Client().CoreV1().Services(test.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	foreignSvcs, err := p.foreignClient.Client().CoreV1().Services(test.NattedNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(homeSvcs.Items) != len(foreignSvcs.Items) {
		return errors.New("home services not correctly reflected remotely")
	}

	for _, svc1 := range homeSvcs.Items {
		var found bool
		for _, svc2 := range foreignSvcs.Items {
			if svc1.Name == svc2.Name {
				found = true
				if !test.AssertServiceCoherency(svc1, svc2) {
					return errors.New("services not matching")
				}
				break
			}
		}
		if !found {
			return errors.New("home services not correctly reflected remotely")
		}
	}
	klog.Infof("TEST - Status coherency after %v asserted", event)
	return nil
}
