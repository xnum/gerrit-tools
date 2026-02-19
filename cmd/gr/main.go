package main

import (
	"fmt"
	"os"

	"github.com/gerrit-ai-review/gerrit-tools/internal/cli"
)

var (
	// Version is set by build flags
	Version = "dev"
)

func main() {
	if err := cli.ExecuteGerritCLI(Version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
