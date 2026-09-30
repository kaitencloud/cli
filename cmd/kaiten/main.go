// Command kaiten is the Kaiten command-line interface.
package main

import "github.com/kaitencloud/cli/internal/cmd"

var (
	version = "dev"
	commit  = "HEAD"
	date    = "unknown"
)

func main() {
	cmd.Execute(version, commit, date)
}
