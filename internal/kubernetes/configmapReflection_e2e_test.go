package kubernetes

import (
	"errors"
	"fmt"
	v1 "github.com/liqoTech/liqo/api/namespaceNattingTable/v1"
	"github.com/liqoTech/liqo/internal/kubernetes/test"
	"github.com/liqoTech/liqo/pkg/crdClient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog"
	"testing"
	"time"
)

func TestHandleConfigmapEvents(t *testing.T) {
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
	w, err := p.foreignClient.Client().CoreV1().ConfigMaps(test.NattedNamespace).Watch(metav1.ListOptions{
		Watch: true,
	})
	if err != nil {
		errChan <- err
		return
	}

	go configmapEventsMonitoring(errChan, createsdDone, updatesDone, deletesDone, ticker, w)
	go cmCreation(p, errChan)

loop:
	for {
		select {
		case <-createsdDone:
			if err := verifyCmConsistency(p, "creation"); err != nil {
				t.Fatal(err)
			}
			go cmUpdate(p, errChan)
		case <-updatesDone:
			if err = verifyCmConsistency(p, "update"); err != nil {
				t.Fatal(err)
			}
			go cmDelete(p, errChan)
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

	if err := verifyCmConsistency(p, "delete"); err != nil {
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

func configmapEventsMonitoring(errChan chan error, createsDone, updatesDone, deletesDone chan struct{}, ticker *time.Ticker, w watch.Interface) {
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

		if creates == len(test.ConfigmapTestCases.InputConfigmaps) && !cc {
			createsDone <- struct{}{}
			cc = true
		}
		if updates == len(test.ConfigmapTestCases.UpdateConfigmaps) && !uc {
			updatesDone <- struct{}{}
			uc = true
		}
		if deletes == len(test.ConfigmapTestCases.DeleteConfigmaps) && !dc {
			close(deletesDone)
			dc = true
			ticker.Stop()
		}

		if creates > len(test.ConfigmapTestCases.InputConfigmaps) {
			errChan <- errors.New("too many create events")
			return
		}
		if updates > len(test.ConfigmapTestCases.UpdateConfigmaps) {
			errChan <- errors.New("too many update events")
			return
		}
		if deletes > len(test.ConfigmapTestCases.DeleteConfigmaps) {
			errChan <- errors.New("too many delete events")
			return
		}
	}
}

func cmCreation(p *KubernetesProvider, chanError chan error) {
	klog.Info("TEST - starting cm creation")
	for _, c := range test.ConfigmapTestCases.InputConfigmaps {
		_, err := p.homeClient.Client().CoreV1().ConfigMaps(test.Namespace).Create(c)
		if err != nil {
			chanError <- err
			return
		}
	}
}

func cmUpdate(p *KubernetesProvider, chanError chan error) {
	klog.Info("TEST - starting cm update")
	for _, c := range test.ConfigmapTestCases.UpdateConfigmaps {
		_, err := p.homeClient.Client().CoreV1().ConfigMaps(test.Namespace).Update(c)
		if err != nil {
			chanError <- err
			return
		}
	}
}

func cmDelete(p *KubernetesProvider, chanError chan error) {
	klog.Info("TEST - starting cm delete")
	for _, c := range test.ConfigmapTestCases.DeleteConfigmaps {
		err := p.homeClient.Client().CoreV1().ConfigMaps(test.Namespace).Delete(c.Name, &metav1.DeleteOptions{})
		if err != nil {
			chanError <- err
			return
		}
	}
}

func verifyCmConsistency(p *KubernetesProvider, event string) error {
	klog.Infof("TEST - Asserting status coherency after %v", event)
	homeCms, err := p.homeClient.Client().CoreV1().ConfigMaps(test.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	foreignCms, err := p.foreignClient.Client().CoreV1().ConfigMaps(test.NattedNamespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(homeCms.Items) != len(foreignCms.Items) {
		return errors.New("home configmaps not correctly reflected remotely")
	}

	for _, cm1 := range homeCms.Items {
		var found bool
		for _, cm2 := range foreignCms.Items {
			if cm1.Name == cm2.Name {
				found = true
				if !test.AssertConfigmapCoherency(cm1, cm2) {
					return errors.New("configmaps not matching")
				}
				break
			}
		}
		if !found {
			return errors.New("home configmaps not correctly reflected remotely")
		}
	}
	klog.Infof("TEST - Status coherency after %v asserted", event)
	return nil
}
