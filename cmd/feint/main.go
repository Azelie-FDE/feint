// Command feint runs a local emulator for European clouds.
//
// It answers Scaleway, Outscale and Exoscale API calls on one port so SDKs,
// CLIs and Terraform can run without an account and without spending anything.
package main

import (
	"os"

	"github.com/stephrobert/feint/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr))
}
