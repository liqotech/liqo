// Copyright 2019-2021 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package uninstaller

import (
	"context"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// WaitForResources waits until existing peerings are disabled and associated resources are removed.
func WaitForResources(client dynamic.Interface) error {
	klog.Info("Waiting for Liqo Resources to be correctly deleted")
	var wg sync.WaitGroup
	deletionResult := make(chan *resultType, len(toCheck))
	conditionResult := make(chan *resultType, ConditionsToCheck)
	wg.Add(len(toCheck) + ConditionsToCheck)
	for _, resource := range toCheck {
		go WaitForEffectiveDeletion(client, resource, deletionResult, &wg, CheckDeletion)
	}
	go WaitForEffectiveDeletion(client, toCheckDeleted{}, conditionResult, &wg, CheckUnjoin)
	wg.Wait()

	close(deletionResult)
	close(conditionResult)

	for elem := range deletionResult {
		if !elem.Success {
			klog.Errorf("Error while waiting for %s to be deleted", elem.Resource.gvr.GroupResource())
			return nil
		}
		printLabels, _ := generateLabelString(elem.Resource.labelSelector)
		klog.Infof("%s instances with \"%s\" labels correctly deleted", elem.Resource.gvr.GroupResource(), printLabels)
	}
	for elem := range conditionResult {
		if !elem.Success {
			klog.Errorf("All peerings have been correctly deleted")
			return nil
		}
	}

	return nil
}

// WaitForEffectiveDeletion waits until toCheck resources are deleted.
func WaitForEffectiveDeletion(client dynamic.Interface, toCheck toCheckDeleted, result chan *resultType, wg *sync.WaitGroup, funcCheck func(client dynamic.Interface, res *resultType, quit chan struct{}, toCheck toCheckDeleted)) {
	defer wg.Done()
	ticker := time.NewTicker(TickerInterval)
	timeout := time.NewTicker(TickerTimeout)
	quit := make(chan struct{})
	var res = &resultType{
		Resource: toCheck,
		Success:  false,
	}
	for {
		select {
		case <-timeout.C:
			close(quit)
		case <-ticker.C:
			funcCheck(client, res, quit, toCheck)
		case <-quit:
			ticker.Stop()
			timeout.Stop()
			result <- res
			return
		}
	}
}

func CheckDeletion(client dynamic.Interface, res *resultType, quit chan struct{}, toCheck toCheckDeleted) {
	value, wError := CheckObjectsDeletion(client, toCheck)
	if value {
		res.Success = true
		close(quit)
	}
	if wError != nil {
		klog.Infof("Error while waiting for deletion of resource %s: %s", toCheck.gvr.GroupResource(), wError.Error())
		close(quit)
	}
	printLabels, _ := generateLabelString(toCheck.labelSelector)
	klog.Infof("Waiting for %s instances with %s labels to be correctly deleted", toCheck.gvr.GroupResource(), printLabels)
}

func CheckUnjoin(client dynamic.Interface, res *resultType, quit chan struct{}, toCheck toCheckDeleted) {
	foreignClusterList, err := getForeignList(client)
	if err != nil {
		close(quit)
	}

	flag := checkPeeringsStatus(foreignClusterList)
	if flag {
		klog.Infof("All incoming or outgoing peering are disabled")
		res.Success = true
		close(quit)
	}
}

// CheckObjectsDeletion verifies that objects of a certain type have been deleted or are not present on the server.
// It returns true when this last condition is verified.
func CheckObjectsDeletion(client dynamic.Interface, objectsToCheck toCheckDeleted) (bool, error) {
	var (
		objectList  *unstructured.UnstructuredList
		err         error
		labelString string
	)

	if labelString, err = generateLabelString(objectsToCheck.labelSelector); err != nil {
		return false, err
	}
	options := metav1.ListOptions{
		LabelSelector: labelString,
	}

	objectList, err = client.Resource(objectsToCheck.gvr).Namespace("").List(context.TODO(), options)

	if apierrors.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	if len(objectList.Items) == 0 {
		klog.V(6).Infof("%s not found", objectsToCheck.gvr)
		return true, nil
	}

	return false, nil
}
