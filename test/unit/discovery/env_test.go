package discovery

import (
	"context"
	protocolv1 "github.com/liqoTech/liqo/api/advertisement-operator/v1"
	policyv1 "github.com/liqoTech/liqo/api/cluster-config/v1"
	v1 "github.com/liqoTech/liqo/api/discovery/v1"
	"github.com/liqoTech/liqo/internal/discovery"
	foreign_cluster_operator "github.com/liqoTech/liqo/internal/discovery/foreign-cluster-operator"
	search_domain_operator "github.com/liqoTech/liqo/internal/discovery/search-domain-operator"
	peering_request_operator "github.com/liqoTech/liqo/internal/peering-request-operator"
	"github.com/liqoTech/liqo/pkg/clusterID"
	"github.com/liqoTech/liqo/pkg/crdClient"
	"github.com/liqoTech/liqo/pkg/liqonet"
	"github.com/miekg/dns"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"net"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strconv"
	"strings"
	"time"
)

type Cluster struct {
	env           *envtest.Environment
	cfg           *rest.Config
	client        *crdClient.CRDClient
	advClient     *crdClient.CRDClient
	discoveryCtrl discovery.DiscoveryCtrl
	fcReconciler  *foreign_cluster_operator.ForeignClusterReconciler
	prReconciler  *peering_request_operator.PeeringRequestReconciler
	sdReconciler  *search_domain_operator.SearchDomainReconciler
	clusterId     *clusterID.ClusterID
}

func getClientCluster() *Cluster {
	cluster, mgr := getCluster()
	cluster.clusterId = clusterID.GetNewClusterID("client-cluster", cluster.client.Client())
	cluster.fcReconciler = foreign_cluster_operator.GetFCReconciler(
		mgr.GetScheme(),
		"default",
		cluster.client,
		cluster.advClient,
		cluster.clusterId,
		1*time.Minute,
		&cluster.discoveryCtrl,
	)
	err := cluster.fcReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.prReconciler = peering_request_operator.GetPRReconciler(
		mgr.GetScheme(),
		cluster.client,
		"default",
		cluster.clusterId,
		"liqo-config",
		"broadcaster",
		"br-sa",
	)
	err = cluster.prReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.discoveryCtrl = discovery.GetDiscoveryCtrl(
		"default",
		cluster.client,
		cluster.advClient,
		cluster.clusterId,
	)

	cluster.sdReconciler = search_domain_operator.GetSDReconciler(
		mgr.GetScheme(),
		cluster.client,
		&cluster.discoveryCtrl,
		1*time.Minute,
	)
	err = cluster.sdReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	go func() {
		err = mgr.Start(stopChan)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}
	}()
	return cluster
}

func getServerCluster() *Cluster {
	cluster, mgr := getCluster()
	cluster.clusterId = clusterID.GetNewClusterID("server-cluster", cluster.client.Client())
	cluster.fcReconciler = foreign_cluster_operator.GetFCReconciler(
		mgr.GetScheme(),
		"default",
		cluster.client,
		cluster.advClient,
		cluster.clusterId,
		1*time.Minute,
		&cluster.discoveryCtrl,
	)
	err := cluster.fcReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.prReconciler = peering_request_operator.GetPRReconciler(
		mgr.GetScheme(),
		cluster.client,
		"default",
		cluster.clusterId,
		"liqo-config",
		"broadcaster",
		"br-sa",
	)
	err = cluster.prReconciler.SetupWithManager(mgr)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.discoveryCtrl = discovery.GetDiscoveryCtrl(
		"default",
		cluster.client,
		cluster.advClient,
		cluster.clusterId,
	)

	go func() {
		err = mgr.Start(stopChan)
		if err != nil {
			klog.Error(err, err.Error())
			os.Exit(1)
		}
	}()
	return cluster
}

func getCluster() (*Cluster, manager.Manager) {
	cluster := &Cluster{}

	cluster.env = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "..", "deployments", "liqo_chart", "crds")},
	}

	/*
		Then, we start the envtest cluster.
	*/
	var err error
	cluster.cfg, err = cluster.env.Start()
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.cfg.ContentConfig.GroupVersion = &v1.GroupVersion
	cluster.cfg.APIPath = "/apis"
	cluster.cfg.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	cluster.cfg.UserAgent = rest.DefaultKubernetesUserAgent()

	advCfg := *cluster.cfg
	advCfg.ContentConfig.GroupVersion = &protocolv1.GroupVersion
	crdClient.AddToRegistry("advertisements", &protocolv1.Advertisement{}, &protocolv1.AdvertisementList{}, nil, protocolv1.GroupResource)

	err = v1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	err = protocolv1.AddToScheme(scheme.Scheme)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	cluster.client, err = crdClient.NewFromConfig(cluster.cfg)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	cluster.advClient, err = crdClient.NewFromConfig(&advCfg)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
	k8sManager, err := ctrl.NewManager(cluster.cfg, ctrl.Options{
		Scheme:             scheme.Scheme,
		MetricsBindAddress: "0", // this avoids port binding collision
	})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
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
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	getLiqoConfig(cluster.client.Client())
	getClusterConfig(*cluster.cfg)

	return cluster, k8sManager
}

func getLiqoConfig(client kubernetes.Interface) {
	// default config values
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "liqo-config",
		},
		Data: map[string]string{
			"clusterID":        "cluster-1",
			"podCIDR":          "10.244.0.0/16",
			"serviceCIDR":      "10.96.0.0/12",
			"gatewayPrivateIP": "10.244.2.47",
			"gatewayIP":        "10.251.0.1",
		},
	}
	_, err := client.CoreV1().ConfigMaps("default").Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func getClusterConfig(config rest.Config) {
	cc := &policyv1.ClusterConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "configuration",
		},
		Spec: policyv1.ClusterConfigSpec{
			AdvertisementConfig: policyv1.AdvertisementConfig{
				AutoAccept:                 true,
				MaxAcceptableAdvertisement: 5,
				ResourceSharingPercentage:  30,
				EnableBroadcaster:          true,
			},
			DiscoveryConfig: policyv1.DiscoveryConfig{
				AutoJoin:            true,
				AutoJoinUntrusted:   true,
				Domain:              "local.",
				EnableAdvertisement: true,
				EnableDiscovery:     true,
				Name:                "MyLiqo",
				Port:                6443,
				AllowUntrustedCA:    false,
				Service:             "_liqo._tcp",
				UpdateTime:          3,
				WaitTime:            2,
				DnsServer:           "8.8.8.8:53",
			},
			LiqonetConfig: policyv1.LiqonetConfig{
				ReservedSubnets: []string{"10.0.0.0/16"},
				PodCIDR:         "192.168.1.1",
				VxlanNetConfig: liqonet.VxlanNetConfig{
					Network:    "",
					DeviceName: "",
					Port:       "",
					Vni:        "",
				},
			},
		},
	}

	config.GroupVersion = &policyv1.GroupVersion
	client, err := crdClient.NewFromConfig(&config)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}

	_, err = client.Resource("clusterconfigs").Create(cc, metav1.CreateOptions{})
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

var registryDomain = "test.liqo.io."
var ptrQueries = map[string][]string{
	registryDomain: {
		"myliqo1." + registryDomain,
		"myliqo2." + registryDomain,
	},
}

type handler struct{}

var hasCname = false

func (h *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	msg.Authoritative = true
	domain := msg.Question[0].Name
	switch r.Question[0].Qtype {
	case dns.TypePTR:
		addresses, ok := ptrQueries[domain]
		if ok {
			for _, address := range addresses {
				msg.Answer = append(msg.Answer, &dns.PTR{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60},
					Ptr: address,
				})
			}
		}
	case dns.TypeSRV:
		var port int
		var host string
		if domain == ptrQueries[registryDomain][0] {
			stringPort := strings.Split(clientCluster.cfg.Host, ":")[1]
			port, _ = strconv.Atoi(stringPort)
			host = "client." + registryDomain
		} else if domain == ptrQueries[registryDomain][1] {
			stringPort := strings.Split(serverCluster.cfg.Host, ":")[1]
			port, _ = strconv.Atoi(stringPort)
			host = "server." + registryDomain
		}
		msg.Answer = append(msg.Answer, &dns.SRV{
			Hdr:      dns.RR_Header{Name: domain, Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 60},
			Priority: 0,
			Weight:   0,
			Port:     uint16(port),
			Target:   host,
		})
	case dns.TypeTXT:
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
			Txt: []string{
				"namespace=default",
			},
		})
		if domain == ptrQueries[registryDomain][0] {
			msg.Answer = append(msg.Answer, &dns.TXT{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
				Txt: []string{
					"id=dns-client-cluster",
				},
			})
		} else if domain == ptrQueries[registryDomain][1] {
			msg.Answer = append(msg.Answer, &dns.TXT{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 60},
				Txt: []string{
					"id=dns-server-cluster",
				},
			})
		}
	case dns.TypeA:
		var host string
		if domain == "client."+registryDomain {
			host = strings.Split(clientCluster.cfg.Host, ":")[0]
		} else if domain == "server."+registryDomain {
			host = strings.Split(serverCluster.cfg.Host, ":")[0]
		}
		if !hasCname {
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
				A:   net.ParseIP(host),
			})
		}
	case dns.TypeCNAME:
		if hasCname {
			msg.Answer = append(msg.Answer, &dns.CNAME{
				Hdr:    dns.RR_Header{Name: domain, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: 60},
				Target: "cname.test.liqo.io.",
			})
		}
	}
	err := w.WriteMsg(&msg)
	if err != nil {
		klog.Error(err, err.Error())
		os.Exit(1)
	}
}

func SetupDNSServer() {
	dnsServer := dns.Server{
		Addr: "127.0.0.1:8053",
		Net:  "udp",
	}
	dnsServer.Handler = &handler{}
	go func() {
		if err := dnsServer.ListenAndServe(); err != nil {
			klog.Fatal("Failed to set udp listener ", err.Error())
		}
	}()
}
