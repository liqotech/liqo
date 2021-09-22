module github.com/liqotech/liqo

go 1.16

require (
	github.com/Azure/azure-sdk-for-go v56.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.19
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.8
	github.com/aws/aws-sdk-go v1.39.4
	github.com/clastix/capsule v0.1.0
	github.com/containernetworking/plugins v0.8.6
	github.com/coreos/go-iptables v0.4.5
	github.com/google/go-cmp v0.5.6
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grandcat/zeroconf v1.0.0
	github.com/gruntwork-io/gruntwork-cli v0.7.0
	github.com/gruntwork-io/terratest v0.35.6
	github.com/jinzhu/copier v0.0.0-20201025035756-632e723a6687
	github.com/julienschmidt/httprouter v1.3.0
	github.com/metal-stack/go-ipam v1.8.4-0.20210322080203-5a9da5064b27
	github.com/miekg/dns v1.1.35
	github.com/mittwald/go-helm-client v0.8.1
	github.com/modern-go/reflect2 v1.0.1
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/pkg/errors v0.9.1
	github.com/spf13/cast v1.4.0 // indirect
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/virtual-kubelet/virtual-kubelet v1.5.1
	github.com/vishvananda/netlink v1.1.0
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae
	go.opencensus.io v0.23.0
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914 // indirect
	golang.org/x/sys v0.0.0-20210817190340-bfb29a6856f2
	golang.org/x/tools v0.1.4 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200609130330-bd2cb7843e1b
	gomodules.xyz/jsonpatch/v2 v2.2.0
	google.golang.org/api v0.48.0
	google.golang.org/genproto v0.0.0-20210624195500-8bfb893ecb84 // indirect
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gotest.tools v2.2.0+incompatible
	helm.sh/helm/v3 v3.6.2
	inet.af/netaddr v0.0.0-20210313195008-843b4240e319
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/component-helpers v0.22.1
	k8s.io/klog/v2 v2.10.0
	k8s.io/kubectl v0.22.1
	k8s.io/metrics v0.22.1
	k8s.io/utils v0.0.0-20210820185131-d34e5cb4466e
	sigs.k8s.io/aws-iam-authenticator v0.5.3
	sigs.k8s.io/controller-runtime v0.10.0
)

replace github.com/virtual-kubelet/virtual-kubelet => github.com/liqotech/virtual-kubelet v1.5.1-0.20210726130647-f2333d82a6de

replace github.com/grandcat/zeroconf => github.com/liqotech/zeroconf v1.0.1-0.20201020081245-6384f3f21ffb

replace github.com/mittwald/go-helm-client v0.8.1 => github.com/liqotech/go-helm-client v0.8.1-0.20210823204019-f878e0fc4d26
