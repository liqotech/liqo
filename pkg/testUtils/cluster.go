package testUtils

import (
	"context"
	"github.com/liqotech/liqo/apis/discovery/v1alpha1"
	nettypes "github.com/liqotech/liqo/apis/net/v1alpha1"
	advtypes "github.com/liqotech/liqo/apis/sharing/v1alpha1"
	"github.com/liqotech/liqo/pkg/crdClient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Cluster struct {
	env       *envtest.Environment
	cfg       *rest.Config
	client    *crdClient.CRDClient
	advClient *crdClient.CRDClient
	netClient *crdClient.CRDClient
}

func (c *Cluster) GetEnv() *envtest.Environment {
	return c.env
}

func (c *Cluster) GetClient() *crdClient.CRDClient {
	return c.client
}

func NewTestCluster() (Cluster, manager.Manager, error) {
	cluster := Cluster{}

	cluster.env = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "deployments", "liqo", "crds")},
	}

	/*
		Then, we start the envtest cluster.
	*/
	var err error
	cluster.cfg, err = cluster.env.Start()
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	cluster.cfg.ContentConfig.GroupVersion = &v1alpha1.GroupVersion
	cluster.cfg.APIPath = "/apis"
	cluster.cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cluster.cfg.UserAgent = rest.DefaultKubernetesUserAgent()

	advCfg := *cluster.cfg
	advCfg.ContentConfig.GroupVersion = &advtypes.GroupVersion
	crdClient.AddToRegistry("advertisements", &advtypes.Advertisement{}, &advtypes.AdvertisementList{}, nil, advtypes.GroupResource)

	netCfg := *cluster.cfg
	netCfg.ContentConfig.GroupVersion = &nettypes.GroupVersion
	crdClient.AddToRegistry("networkconfigs", &nettypes.NetworkConfig{}, &nettypes.NetworkConfigList{}, nil, nettypes.TunnelEndpointGroupResource)
	crdClient.AddToRegistry("tunnelendpoints", &nettypes.TunnelEndpoint{}, &nettypes.TunnelEndpointList{}, nil, nettypes.TunnelEndpointGroupResource)

	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	err = advtypes.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	err = nettypes.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	cluster.client, err = crdClient.NewFromConfig(cluster.cfg)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	cluster.advClient, err = crdClient.NewFromConfig(&advCfg)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	cluster.netClient, err = crdClient.NewFromConfig(&netCfg)
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}
	k8sManager, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	// creates empty CaData secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "ca-data",
		},
		Data: map[string][]byte{
			"ca.crt": []byte(""),
		},
	}
	_, err = cluster.client.Client().CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err)
		return Cluster{}, nil, err
	}

	return cluster, k8sManager, nil
}
