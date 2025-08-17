package main

import (
	"fmt"
	"os"
	"splunk_cli/cmd"
)

// These variables are set by the linker.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func main() {
	// Manual check for the --version flag
	for _, arg := range os.Args {
		if arg == "--version" {
			fmt.Printf("splunk-cli version %s\ncommit %s\nbuilt at %s\n", Version, Commit, Date)
			os.Exit(0)
		}
	}

	cmd.Execute()
}