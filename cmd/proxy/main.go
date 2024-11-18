package main

import (
	"context"
	"flag"
	"os"

	"github.com/liqotech/liqo/pkg/proxy"
	"k8s.io/klog/v2"
)

func main() {
	ctx := context.Background()

	port := flag.Int("port", 8080, "port to listen on")
	allowedHosts := flag.String("allowed-hosts", "", "comma separated list of allowed hosts")

	flag.Parse()

	p := proxy.New(*allowedHosts, *port)

	if err := p.Start(ctx); err != nil {
		klog.Error(err)
		os.Exit(1)
	}
}
