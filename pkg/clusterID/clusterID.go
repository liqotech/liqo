package clusterID

import (
	"errors"
	"github.com/go-logr/logr"
	"github.com/liqoTech/liqo/internal/discovery/clients"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sort"
	"strings"
	"sync"
)

type ClusterID struct {
	id string
	m  sync.RWMutex

	client *kubernetes.Clientset
	log    logr.Logger
}

func GetNewClusterID(id string, client *kubernetes.Clientset, log logr.Logger) *ClusterID {
	return &ClusterID{
		id:     id,
		m:      sync.RWMutex{},
		client: client,
		log:    log,
	}
}

func NewClusterID() (*ClusterID, error) {
	client, err := clients.NewK8sClient()
	if err != nil {
		return nil, err
	}
	clusterId := &ClusterID{
		client: client,
		log:    ctrl.Log.WithName("cluster-id"),
	}

	namespace, found := os.LookupEnv("POD_NAMESPACE")
	if !found {
		clusterId.log.Info("POD_NAMESPACE not set")
		data, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			clusterId.log.Error(err, "Unable to get namespace")
			os.Exit(1)
		}
		if namespace = strings.TrimSpace(string(data)); len(namespace) <= 0 {
			clusterId.log.Error(err, "Unable to get namespace")
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
		return err
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
	nodes, err := cId.client.CoreV1().Nodes().List(metav1.ListOptions{
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
	cm, err := cId.client.CoreV1().ConfigMaps(namespace).Get("cluster-id", metav1.GetOptions{})
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
			_, err := cId.client.CoreV1().ConfigMaps(namespace).Create(cm)
			return err
		}
		// other errors
		return err
	}
	// already exists, update it if needed
	if cm.Data["cluster-id"] != id {
		cm.Data["cluster-id"] = id
		_, err := cId.client.CoreV1().ConfigMaps(namespace).Update(cm)
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
	cId.log.Info("ClusterID: " + tmp)
}
