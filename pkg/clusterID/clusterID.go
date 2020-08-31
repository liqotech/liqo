package clusterID

import (
	"context"
	"errors"
	"github.com/google/uuid"
	discoveryv1alpha1 "github.com/liqoTech/liqo/api/discovery/v1alpha1"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"os"
	"sort"
	"strings"
	"sync"
)

type ClusterID struct {
	id string
	m  sync.RWMutex

	client kubernetes.Interface
}

func GetNewClusterID(id string, client kubernetes.Interface) *ClusterID {
	return &ClusterID{
		id:     id,
		m:      sync.RWMutex{},
		client: client,
	}
}

func NewClusterID(kubeconfigPath string) (*ClusterID, error) {
	config, err := crdClient.NewKubeconfig(kubeconfigPath, &discoveryv1alpha1.GroupVersion)
	if err != nil {
		klog.Error(err, "unable to get kube config")
		os.Exit(1)
	}
	crdClientVar, err := crdClient.NewFromConfig(config)
	if err != nil {
		klog.Error(err, "unable to create crd client")
		os.Exit(1)
	}
	clusterId := &ClusterID{
		client: crdClientVar.Client(),
	}

	namespace, found := os.LookupEnv("POD_NAMESPACE")
	if !found {
		klog.Info("POD_NAMESPACE not set")
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			klog.Error(err, "Unable to get namespace")
			os.Exit(1)
		}
		if namespace = strings.TrimSpace(string(data)); len(namespace) <= 0 {
			klog.Error(err, "Unable to get namespace")
			os.Exit(1)
		}
	}

	watchlist := cache.NewListWatchFromClient(
		clusterId.client.CoreV1().RESTClient(),
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
				clusterId.clusterIdUpdated(obj)
			},
			DeleteFunc: func(obj interface{}) {
				clusterId.m.Lock()
				clusterId.id = ""
				clusterId.m.Unlock()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				clusterId.clusterIdUpdated(newObj)
			},
		},
	)

	go func() {
		stop := make(chan struct{})
		defer close(stop)
		controller.Run(stop)
	}()

	return clusterId, nil
}

func (cId *ClusterID) SetupClusterID(namespace string) error {
	id, err := cId.getMasterID()
	if err != nil {
		klog.Warning(err, "cannot get UID from master, generating new one")
		id = uuid.New().String()
	}
	err = cId.save(id, namespace)
	if err != nil {
		return err
	}
	return nil
}

func (cId *ClusterID) GetClusterID() string {
	cId.m.RLock()
	res := cId.id
	cId.m.RUnlock()
	return res
}

func (cId *ClusterID) getMasterID() (string, error) {
	nodes, err := cId.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
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

func (cId *ClusterID) save(id string, namespace string) error {
	cm, err := cId.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "cluster-id", metav1.GetOptions{})
	if err != nil {
		if k8serror.IsNotFound(err) {
			// does not exist
			cm = &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster-id",
				},
				Data: map[string]string{
					"cluster-id": id,
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
	if cm.Data["cluster-id"] != id {
		cm.Data["cluster-id"] = id
		_, err := cId.client.CoreV1().ConfigMaps(namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		return err
	}
	return nil
}

func (cId *ClusterID) clusterIdUpdated(obj interface{}) {
	tmp := obj.(*v1.ConfigMap).Data["cluster-id"]
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
