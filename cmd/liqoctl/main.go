package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	liqocmd "github.com/liqotech/liqo/cmd/liqoctl/cmd"
)

const (
	terminationTimeout = 5 * time.Second
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-ctx.Done()
		<-time.After(terminationTimeout)
		os.Exit(1)
	}()

	cmd := liqocmd.NewRootCommand(ctx)
	cobra.CheckErr(cmd.ExecuteContext(ctx))
}
