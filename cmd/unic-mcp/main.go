package main

import (
	"context"
	"fmt"
	"os"

	"unic/internal/cli"
	"unic/internal/mcp"
)

func main() {
	server := mcp.New(cli.Version, cli.ExecuteAutomation)
	if err := server.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
