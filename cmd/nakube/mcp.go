package main

import (
	"context"
	"fmt"
	"os"

	"github.com/dacsang97/nakube/internal/cluster"
	"github.com/dacsang97/nakube/internal/mcp"
)

func runMCP() {
	cluster.PreparePath()
	if err := cluster.ImportShellEnv(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	session := cluster.NewSession()
	session.RefreshLoadingRules()
	if err := session.Open(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	server := &mcp.Server{API: mcp.SessionAPI{S: session}}
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
