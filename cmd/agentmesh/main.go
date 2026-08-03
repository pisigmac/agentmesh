package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentmesh/agentmesh/adapters"
	"github.com/agentmesh/agentmesh/pkg/agentmesh"
)

func main() {
	base := flag.String("base", "http://localhost:8080", "control plane base URL")
	key := flag.String("key", "agentmesh-dev-key", "api key")
	org := flag.String("org", "default", "org id")
	agentType := flag.String("type", adapters.TypeCursor, "agent type")
	agentID := flag.String("id", "cursor-1", "agent id")
	endpoint := flag.String("endpoint", "http://127.0.0.1:9999", "agent endpoint")
	persistent := flag.Bool("persistent", false, "store registration in etcd without lease expiry")
	keepalive := flag.Bool("keepalive", false, "re-register on interval (for leased registrations)")
	interval := flag.Duration("interval", 15*time.Second, "keepalive re-register interval")
	flag.Parse()

	client := agentmesh.New(agentmesh.Config{BaseURL: *base, APIKey: *key, OrgID: *org})
	register := func() error {
		return adapters.Register(context.Background(), client, *agentType, *agentID, *endpoint, nil, *persistent)
	}
	if err := register(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("registered", *agentID, "persistent=", *persistent)

	if !*keepalive || *persistent {
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := register(); err != nil {
				fmt.Fprintln(os.Stderr, "keepalive:", err)
			}
		}
	}
}
