module github.com/liqotech/liqo

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	contrib.go.opencensus.io/exporter/ocagent v0.7.0
	github.com/containernetworking/plugins v0.8.6
	github.com/coreos/go-iptables v0.4.5
	github.com/go-logr/logr v0.4.0
	github.com/go-logr/zapr v0.4.0 // indirect
	github.com/google/go-cmp v0.5.3
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.7.4
	github.com/grandcat/zeroconf v1.0.0
	github.com/gruntwork-io/terratest v0.30.7
	github.com/jinzhu/copier v0.0.0-20201025035756-632e723a6687
	github.com/julienschmidt/httprouter v1.3.0
	github.com/metal-stack/go-ipam v1.8.4-0.20210322080203-5a9da5064b27
	github.com/miekg/dns v1.1.27
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/modern-go/reflect2 v1.0.1
	github.com/onsi/ginkgo v1.15.2
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/prometheus/common v0.15.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f
	go.opencensus.io v0.23.0
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.16.0 // indirect
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/mod v0.4.2 // indirect
	golang.org/x/sys v0.0.0-20210317225723-c4fcb01b228e
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/tools v0.1.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	gotest.tools v2.2.0+incompatible
	honnef.co/go/tools v0.1.3 // indirect
	inet.af/netaddr v0.0.0-20210313195008-843b4240e319
	k8s.io/api v0.20.1
	k8s.io/apiextensions-apiserver v0.19.4 // indirect
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/kubectl v0.18.6
	k8s.io/kubernetes v1.18.6
	k8s.io/metrics v0.18.6
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
	sigs.k8s.io/controller-runtime v0.6.2
)

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.6

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.6

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.6

replace k8s.io/apiserver => k8s.io/apiserver v0.18.6

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.6

replace k8s.io/cri-api => k8s.io/cri-api v0.18.6

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.6

replace k8s.io/kubelet => k8s.io/kubelet v0.18.6

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.6

replace k8s.io/apimachinery => k8s.io/apimachinery v0.18.6

replace k8s.io/api => k8s.io/api v0.18.6

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.6

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.6

replace k8s.io/component-base => k8s.io/component-base v0.18.6

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.6

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.6

replace k8s.io/metrics => k8s.io/metrics v0.18.6

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.6

replace k8s.io/code-generator => k8s.io/code-generator v0.18.6

replace k8s.io/client-go => k8s.io/client-go v0.18.6

replace k8s.io/kubectl => k8s.io/kubectl v0.18.6

replace k8s.io/node-api => k8s.io/node-api v0.18.6

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.18.6

replace k8s.io/sample-controller => k8s.io/sample-controller v0.18.6

replace github.com/grandcat/zeroconf => github.com/liqotech/zeroconf v1.0.1-0.20201020081245-6384f3f21ffb
