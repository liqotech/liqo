module github.com/liqotech/liqo

go 1.16

require (
	github.com/aws/aws-sdk-go v1.39.4
	github.com/clastix/capsule v0.1.0-rc2
	github.com/containernetworking/plugins v0.8.6
	github.com/coreos/go-iptables v0.4.5
	github.com/daixiang0/gci v0.2.9 // indirect
	github.com/google/go-cmp v0.5.5
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.2.0
	github.com/gorilla/mux v1.8.0
	github.com/grandcat/zeroconf v1.0.0
	github.com/gruntwork-io/gruntwork-cli v0.7.0
	github.com/gruntwork-io/terratest v0.35.6
	github.com/jinzhu/copier v0.0.0-20201025035756-632e723a6687
	github.com/julienschmidt/httprouter v1.3.0
	github.com/metal-stack/go-ipam v1.8.4-0.20210322080203-5a9da5064b27
	github.com/miekg/dns v1.1.35
	github.com/mitchellh/go-homedir v1.1.0
	github.com/modern-go/reflect2 v1.0.1
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/virtual-kubelet/virtual-kubelet v1.5.1
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae
	go.opencensus.io v0.23.0
	go.uber.org/goleak v1.1.10
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914 // indirect
	golang.org/x/sys v0.0.0-20210603081109-ebe580a85c40
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	golang.org/x/tools v0.1.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
	gomodules.xyz/jsonpatch/v2 v2.1.0
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.26.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gotest.tools v2.2.0+incompatible
	inet.af/netaddr v0.0.0-20210313195008-843b4240e319
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/component-helpers v0.21.0
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/kubectl v0.21.0
	k8s.io/kubelet v0.0.0
	k8s.io/kubernetes v1.21.0
	k8s.io/metrics v0.21.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/aws-iam-authenticator v0.5.3
	sigs.k8s.io/controller-runtime v0.9.0-beta.2
)

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.21.0

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.21.0

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.21.0

replace k8s.io/apiserver => k8s.io/apiserver v0.21.0

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.21.0

replace k8s.io/cri-api => k8s.io/cri-api v0.21.0

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.21.0

replace k8s.io/kubelet => k8s.io/kubelet v0.21.0

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.21.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.21.0

replace k8s.io/api => k8s.io/api v0.21.0

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.21.0

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.21.0

replace k8s.io/component-base => k8s.io/component-base v0.21.0

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.21.0

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.21.0

replace k8s.io/metrics => k8s.io/metrics v0.21.0

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.21.0

replace k8s.io/code-generator => k8s.io/code-generator v0.21.0

replace k8s.io/client-go => k8s.io/client-go v0.21.0

replace k8s.io/kubectl => k8s.io/kubectl v0.21.0

replace k8s.io/node-api => k8s.io/node-api v0.21.0

replace k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.21.0

replace k8s.io/sample-controller => k8s.io/sample-controller v0.21.0

replace github.com/grandcat/zeroconf => github.com/liqotech/zeroconf v1.0.1-0.20201020081245-6384f3f21ffb

replace k8s.io/component-helpers => k8s.io/component-helpers v0.21.0

replace k8s.io/controller-manager => k8s.io/controller-manager v0.21.0

replace k8s.io/mount-utils => k8s.io/mount-utils v0.21.0

replace github.com/virtual-kubelet/virtual-kubelet => github.com/liqotech/virtual-kubelet v1.5.1-0.20210726130647-f2333d82a6de
