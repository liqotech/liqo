package namespacesMapping

import (
	"context"
	"errors"
	nattingv1 "github.com/liqotech/liqo/api/virtualKubelet/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	v1 "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
	"strings"
)

type namespaceNTCache struct {
	Store            cache.Store
	Controller       chan struct{}
	nattingTableName string
}

type NamespaceMapper struct {
	homeClient    crdClient.NamespacedCRDClientInterface
	foreignClient kubernetes.Interface

	cache            namespaceNTCache
	foreignClusterId string
	homeClusterId    string

	startOutgoingReflection chan string
	startIncomingReflection chan string
	stopOutgoingReflection  chan string
	stopIncomingReflection  chan string
	startMapper             chan struct{}
	stopMapper              chan struct{}
	restartReady            chan struct{}
}

func (m *NamespaceMapper) startNattingCache(clientSet crdClient.NamespacedCRDClientInterface) error {
	var err error

	ehf := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			m.startMapper <- struct{}{}
			m.manageReflections(nil, obj)
		},
		UpdateFunc: m.manageReflections,
		DeleteFunc: func(obj interface{}) {
			m.stopMapper <- struct{}{}
			<-m.restartReady
			if err := m.createNattingTable(m.foreignClusterId); err != nil {
				klog.Error(err, "cannot create nattingTable")
			}
		},
	}
	lo := metav1.ListOptions{FieldSelector: strings.Join([]string{"metadata.name", m.cache.nattingTableName}, "=")}

	m.cache.Store, m.cache.Controller, err = crdClient.WatchResources(clientSet,
		"namespacenattingtables", "",
		0, ehf, lo)
	if err != nil {
		return err
	}
	klog.Info("namespaceNattingTable cache initialized")

	return nil
}

func (nt *namespaceNTCache) WaitNamespaceNattingTableSync() {
	cache.WaitForCacheSync(nt.Controller, func() bool {
		_, exists, _ := nt.Store.GetByKey(nt.nattingTableName)
		return exists
	})
}

func (m *NamespaceMapper) NatNamespace(namespace string, create bool) (string, error) {
	nt, exists, err := m.cache.Store.GetByKey(m.foreignClusterId)
	if err != nil {
		return "", err
	}

	if !exists {
		return "", errors.New("namespacenattingtable not existing")
	}

	nattingTable := nt.(*nattingv1.NamespaceNattingTable)
	nattedNS, ok := nattingTable.Spec.NattingTable[namespace]
	if !ok && !create {
		return "", errors.New("not natted namespaces")
	}

	if !ok && create {
		nattedNS = strings.Join([]string{namespace, m.homeClusterId}, "-")
		if nattingTable.Spec.NattingTable == nil {
			nattingTable.Spec.NattingTable = make(map[string]string)
			nattingTable.Spec.DeNattingTable = make(map[string]string)
		}

		nattingTable.Spec.NattingTable[namespace] = nattedNS
		nattingTable.Spec.DeNattingTable[nattedNS] = namespace

		_, err := m.homeClient.Resource("namespacenattingtables").Update(nattingTable.Name, nattingTable, metav1.UpdateOptions{})
		if err != nil {
			return "", err
		}

		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nattedNS,
			},
		}

		_, err = m.foreignClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
		if err != nil && !kerror.IsAlreadyExists(err) {
			return "", err
		}
	}

	return nattedNS, nil
}

func (m *NamespaceMapper) DeNatNamespace(namespace string) (string, error) {
	nt, exists, err := m.cache.Store.GetByKey(m.foreignClusterId)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", errors.New("namespacenattingtable not existing")
	}

	deNattedNS, ok := nt.(*nattingv1.NamespaceNattingTable).Spec.DeNattingTable[namespace]
	if !ok {
		return "", errors.New("not natted namespaces")
	}

	return deNattedNS, nil
}

func (m *NamespaceMapper) getMappedNamespaces() (map[string]string, error) {
	obj, exists, err := m.cache.Store.GetByKey(m.foreignClusterId)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.New("namespacenattingtable not existing")
	}
	nt := obj.(*nattingv1.NamespaceNattingTable)

	return nt.Spec.NattingTable, nil
}

func (m *NamespaceMapper) createNattingTable(name string) error {
	_, err := m.homeClient.Resource("namespacenattingtables").Get(name, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !kerror.IsNotFound(err) {
		return err
	}

	table := &nattingv1.NamespaceNattingTable{
		TypeMeta: metav1.TypeMeta{
			Kind: "NamespaceNattingTable",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: nattingv1.NamespaceNattingTableSpec{
			ClusterId:    name,
			NattingTable: map[string]string{},
		},
	}

	_, err = m.homeClient.Resource("namespacenattingtables").Create(table, metav1.CreateOptions{})

	if err != nil && kerror.IsAlreadyExists(err) {
		return nil
	}

	klog.Info("new namespaceNattingTable created")

	return err
}

func (m *NamespaceMapper) manageReflections(oldObj interface{}, newObj interface{}) {
	var oldNattingTable map[string]string

	newNattingTable := newObj.(*nattingv1.NamespaceNattingTable).Spec.NattingTable
	if oldObj != nil {
		oldNattingTable = oldObj.(*nattingv1.NamespaceNattingTable).Spec.NattingTable
	}

	for localNs, remoteNs := range newNattingTable {
		if _, ok := oldNattingTable[localNs]; !ok {

			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: remoteNs,
				},
			}

			_, err := m.foreignClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			if err != nil && !kerror.IsAlreadyExists(err) {
				klog.Error(err, "error in namespace creation")
				continue
			}
		}

		m.startOutgoingReflection <- localNs
		m.startIncomingReflection <- localNs
	}

	for localNs := range oldNattingTable {
		if _, ok := newNattingTable[localNs]; !ok {
			m.stopOutgoingReflection <- localNs
			m.stopIncomingReflection <- localNs
		}
	}
}
