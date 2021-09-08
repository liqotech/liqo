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

package clusterid

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/liqotech/liqo/pkg/consts"
	"github.com/liqotech/liqo/pkg/utils"
)

// ClusterID defines the basic methods to interact with cluster identifier.
type ClusterID interface {
	SetupClusterID(namespace string) error
	GetClusterID() string
}

// ClusterIDImpl implements the basic structure to safely manipulate ClusterID.
type ClusterIDImpl struct {
	id string
	m  sync.RWMutex

	client kubernetes.Interface
}

// NewClusterIDFromClient generates a new clusterid and returns it.
func NewClusterIDFromClient(client kubernetes.Interface) (ClusterID, error) {
	newClusterID := &ClusterIDImpl{
		client: client,
	}

	namespace, err := utils.RetrieveNamespace()
	if err != nil {
		return nil, err
	}

	watchlist := cache.NewListWatchFromClient(
		newClusterID.client.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		fields.SelectorFromSet(fields.Set{
			"metadata.name": "cluster-id",
		}),
	)
	_, controller := cache.NewInformer(
		watchlist,
		&v1.ConfigMap{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				newClusterID.updateClusterID(obj)
			},
			DeleteFunc: func(obj interface{}) {
				newClusterID.m.Lock()
				newClusterID.id = ""
				newClusterID.m.Unlock()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				newClusterID.updateClusterID(newObj)
			},
		},
	)

	go func() {
		stop := make(chan struct{})
		defer close(stop)
		controller.Run(stop)
	}()

	return newClusterID, nil
}

func getClusterID(cm *v1.ConfigMap) string {
	if cm == nil {
		return ""
	}
	return cm.Data[consts.ClusterIDConfigMapKey]
}

// SetupClusterID sets a new clusterid.
func (cId *ClusterIDImpl) SetupClusterID(namespace string) error {
	cm, err := cId.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), consts.ClusterIDConfigMapName,
		metav1.GetOptions{})
	if err != nil && !k8serror.IsNotFound(err) {
		klog.Error(err)
		return err
	}
	if k8serror.IsNotFound(err) || getClusterID(cm) == "" {
		id, err := cId.getMasterID()
		if err != nil {
			klog.Warning(err, "cannot get UID from master, generating new one")
			id = uuid.New().String()
		}
		err = cId.saveToConfigMap(id, namespace)
		if err != nil {
			return err
		}
		return nil
	}
	cId.id = getClusterID(cm)
	return nil
}

// GetClusterID retrieves the clusterid.
func (cId *ClusterIDImpl) GetClusterID() string {
	cId.m.RLock()
	res := cId.id
	cId.m.RUnlock()
	return res
}

func (cId *ClusterIDImpl) getMasterID() (string, error) {
	nodes, err := cId.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: consts.MasterLabel,
	})
	if err != nil {
		return "", err
	}
	if len(nodes.Items) == 0 {
		return "", errors.New("no master node found")
	}
	// get first master (ordered alphabetically by name)
	sort.Slice(nodes.Items, func(i, j int) bool {
		return nodes.Items[i].Name < nodes.Items[j].Name
	})
	return string(nodes.Items[0].UID), nil
}

// saveToConfigMap stores the clusterid in the detailed configMap.
func (cId *ClusterIDImpl) saveToConfigMap(id, namespace string) error {
	cm, err := cId.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), consts.ClusterIDConfigMapName,
		metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			// does not exist
			cm = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: consts.ClusterIDConfigMapName,
				},
				Data: map[string]string{
					consts.ClusterIDConfigMapKey: id,
				},
			}
			cId.id = id
			_, err := cId.client.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
			return err
		}
		// other errors
		return err
	}
	// already exists, update it if needed
	if cm.Data[consts.ClusterIDConfigMapKey] != id {
		cm.Data[consts.ClusterIDConfigMapKey] = id
		_, err := cId.client.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		return err
	}
	return nil
}

// updateClusterID updates the clusterid values.
func (cId *ClusterIDImpl) updateClusterID(obj interface{}) {
	tmp := obj.(*v1.ConfigMap).Data[consts.ClusterIDConfigMapKey]
	cId.m.RLock()
	curr := cId.id
	if curr != tmp {
		cId.m.RLocker().Lock()
		cId.id = tmp
		cId.m.RLocker().Unlock()
	}
	cId.m.RUnlock()
	klog.Info("ClusterID: " + tmp)
}
