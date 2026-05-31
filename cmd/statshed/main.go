// Command statshed is the StatShed status-dashboard CLI.
package main

import (
	"os"

	"github.com/statshed/statshed-cli/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
