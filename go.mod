module github.com/liqotech/liqo

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	contrib.go.opencensus.io/exporter/ocagent v0.7.0
	github.com/agrison/go-commons-lang v0.0.0-20200208220349-58e9fcb95174
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/atotto/clipboard v0.1.2
	github.com/coreos/go-iptables v0.4.5
	github.com/gen2brain/beeep v0.0.0-20200526185328-e9c15c258e28
	github.com/gen2brain/dlgs v0.0.0-20201118155338-03fe7f81ad25
	github.com/getlantern/systray v1.1.0
	github.com/go-logr/logr v0.1.0
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/google/go-cmp v0.5.1
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.4
	github.com/grandcat/zeroconf v1.0.0
	github.com/gruntwork-io/terratest v0.30.7
	github.com/jinzhu/copier v0.0.0-20201025035756-632e723a6687
	github.com/joho/godotenv v1.3.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/miekg/dns v1.1.27
	github.com/mitchellh/go-homedir v1.1.0
	github.com/modern-go/reflect2 v1.0.1
	github.com/oleiade/lane v1.0.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.3
	github.com/ozgio/strutil v0.3.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/prometheus/common v0.15.0 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/vishvananda/netlink v1.1.0
	go.opencensus.io v0.22.4
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/sys v0.0.0-20201223074533-0d417f636930
	golang.org/x/term v0.0.0-20201210144234-2321bbc49cbf // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/tools v0.0.0-20201116002733-ac45abd4c88c
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.19.4
	k8s.io/apiextensions-apiserver v0.19.4 // indirect
	k8s.io/apimachinery v0.19.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.0.0
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
