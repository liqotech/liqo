package main

import (
	"flag"

	"github.com/liqotech/liqo/pkg/proxy"
	"k8s.io/klog/v2"
)

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	allowedHosts := flag.String("allowed-hosts", "", "comma separated list of allowed hosts")

	flag.Parse()

	p := proxy.New(*allowedHosts)

	if err := p.SetupProxy(*port); err != nil {
		klog.Error(err)
	}
}
