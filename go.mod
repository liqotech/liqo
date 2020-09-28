module github.com/liqotech/liqo

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	contrib.go.opencensus.io/exporter/ocagent v0.7.0
	github.com/agrison/go-commons-lang v0.0.0-20200208220349-58e9fcb95174
	github.com/apparentlymart/go-cidr v1.1.0
	github.com/atotto/clipboard v0.1.2
	github.com/certifi/gocertifi v0.0.0-20200104152315-a6d78f326758 // indirect
	github.com/cloudflare/cfssl v1.4.1 // indirect
	github.com/cloudflare/redoctober v0.0.0-20200117180338-34d894fcc2a1 // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.18+incompatible // indirect
	github.com/coreos/go-iptables v0.4.5
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/fatih/color v1.9.0 // indirect
	github.com/gen2brain/beeep v0.0.0-20200420150314-13046a26d502
	github.com/gen2brain/dlgs v0.0.0-20200211102745-b9c2664df42f
	github.com/getlantern/systray v0.0.0-20200324212034-d3ab4fd25d99
	github.com/getsentry/raven-go v0.2.0 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/golangci/gocyclo v0.0.0-20180528144436-0a533e8fa43d // indirect
	github.com/golangci/golangci-lint v1.23.1 // indirect
	github.com/golangci/revgrep v0.0.0-20180812185044-276a5c0a1039 // indirect
	github.com/google/certificate-transparency-go v1.1.0 // indirect
	github.com/google/go-cmp v0.5.1
	github.com/google/monologue v0.0.0-20200117164337-ad3ddc05419e // indirect
	github.com/google/uuid v1.1.1
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/websocket v1.4.1 // indirect
	github.com/grandcat/zeroconf v1.0.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0 // indirect
	github.com/jirfag/go-printf-func-name v0.0.0-20200119135958-7558a9eaa5af // indirect
	github.com/joho/godotenv v1.3.0
	github.com/lib/pq v1.3.0 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-runewidth v0.0.8 // indirect
	github.com/mattn/go-sqlite3 v2.0.2+incompatible // indirect
	github.com/miekg/dns v1.1.27
	github.com/mitchellh/go-homedir v1.1.0
	github.com/olekukonko/tablewriter v0.0.4 // indirect
	github.com/ozgio/strutil v0.3.0
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.14.0
	github.com/securego/gosec v0.0.0-20200121091311-459e2d3e91bd // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.2 // indirect
	github.com/streadway/amqp v0.0.0-20200108173154-1c71cc93ed71 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/tmc/grpc-websocket-proxy v0.0.0-20200122045848-3419fae592fc // indirect
	github.com/tommy-muehle/go-mnd v1.2.0 // indirect
	github.com/urfave/cli v1.22.2 // indirect
	github.com/virtual-kubelet/virtual-kubelet v1.2.1 // indirect
	github.com/vishvananda/netlink v1.1.0
	github.com/weppos/publicsuffix-go v0.10.0 // indirect
	github.com/zmap/zlint v1.1.0 // indirect
	go.etcd.io/etcd v3.3.18+incompatible // indirect
	go.opencensus.io v0.22.4
	go.uber.org/atomic v1.5.1 // indirect
	go.uber.org/multierr v1.4.0 // indirect
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae
	golang.org/x/tools v0.0.0-20200616195046-dc31b401abb5
	gopkg.in/ini.v1 v1.51.1 // indirect
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.0.0
	k8s.io/kubectl v0.18.6
	k8s.io/kubernetes v1.18.6
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
	mvdan.cc/unparam v0.0.0-20191111180625-960b1ec0f2c2 // indirect
	sigs.k8s.io/controller-runtime v0.6.2
	sigs.k8s.io/controller-tools v0.4.0 // indirect
	sourcegraph.com/sqs/pbtypes v1.0.0 // indirect
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
