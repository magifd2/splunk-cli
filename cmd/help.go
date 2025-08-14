package cmd

import (
	"flag"
	"fmt"
	"os"

	"splunk_cli/splunk"
)

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: splunk-cli [global options] <command> [options]")
	fmt.Fprintln(os.Stderr, "\nA flexible CLI tool to interact with the Splunk REST API.")
	fmt.Fprintln(os.Stderr, "\nGlobal Options:")
	fmt.Fprintln(os.Stderr, "  --config <path>  Path to a custom configuration file")
	fmt.Fprintln(os.Stderr, "  --version        Print version information and exit")
	fmt.Fprintln(os.Stderr, "\nCommands:")
	fmt.Fprintln(os.Stderr, "  run      Run a search job synchronously and wait for results.")
	fmt.Fprintln(os.Stderr, "  start    Start a search job and print the SID immediately.")
	fmt.Fprintln(os.Stderr, "  status   Check the status of a running search job.")
	fmt.Fprintln(os.Stderr, "  results  Get the results of a completed search job.")
	fmt.Fprintln(os.Stderr, "  help     Show help for a specific command.")
	fmt.Fprintln(os.Stderr, "\nUse 'splunk-cli help <command>' for more information about a specific command.")
}

func printHelp(args []string) {
	if len(args) == 0 {
		printUsage()
		return
	}
	cmd := args[0]
	var fs *flag.FlagSet
	dummyCfg := splunk.Config{}

	// Create a global FlagSet to include --config and --version for help output
	globalFs := flag.NewFlagSet("global", flag.ContinueOnError)
	globalFs.String("config", "", "Path to a custom configuration file")
	globalFs.Bool("version", false, "Print version information and exit") // Also include version here for consistency

	switch cmd {
	case "run":
		fs = flag.NewFlagSet("run", flag.ExitOnError)
		fs.String("spl", "", "SPL query to execute (cannot be used with --file)")
		fs.String("file", "", "Read SPL from a file ('-' for stdin)")
		fs.String("f", "", "Shorthand for --file")
		fs.String("earliest", "", "Search earliest time")
		fs.String("latest", "", "Search latest time")
		fs.Duration("timeout", 0, "Timeout for the run command")
		fs.Bool("silent", false, "Suppress progress messages")
	case "start":
		fs = flag.NewFlagSet("start", flag.ExitOnError)
		fs.String("spl", "", "SPL query to execute (cannot be used with --file)")
		fs.String("file", "", "Read SPL from a file ('-' for stdin)")
		fs.String("f", "", "Shorthand for --file")
		fs.String("earliest", "", "Search earliest time")
		fs.String("latest", "", "Search latest time")
		fs.Bool("silent", false, "Suppress progress messages")
	case "status":
		fs = flag.NewFlagSet("status", flag.ContinueOnError)
		fs.String("sid", "", "Search ID (SID) of the job")
	case "results":
		fs = flag.NewFlagSet("results", flag.ContinueOnError)
		fs.String("sid", "", "Search ID (SID) of the job")
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command for help: %s", cmd)
		return
	}
	addCommonFlags(fs, &dummyCfg)
	fmt.Fprintf(os.Stderr, "Usage: splunk-cli %s [options]\n\nOptions for %s:\n", cmd, cmd)
	fs.PrintDefaults()
	fmt.Fprintln(os.Stderr, "\nGlobal Options:") // Print global options after command-specific ones
	globalFs.PrintDefaults()
}
