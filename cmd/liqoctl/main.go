package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	liqocmd "github.com/liqotech/liqo/cmd/liqoctl/cmd"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	cmd := liqocmd.NewRootCommand(ctx)
	cobra.CheckErr(cmd.Execute())
}
