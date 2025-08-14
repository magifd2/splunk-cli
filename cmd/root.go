package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"splunk_cli/splunk"
)

// These variables are set by the linker.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Execute() {
	var showVersion bool
	var configPath string

	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.StringVar(&configPath, "config", "", "Path to a custom configuration file")
	flag.Parse()

	if showVersion {
		fmt.Printf("splunk-cli version %s\ncommit %s\nbuilt at %s", Version, Commit, Date)
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	log := &splunk.Logger{}
	baseCfg, cfgPath, err := splunk.LoadConfigFromFile(configPath)
	if err != nil {
		log.Printf("Warning: could not load config file at %s: %v", cfgPath, err)
	}

	if baseCfg.HTTPTimeout == 0 {
		baseCfg.HTTPTimeout = 30 * time.Second
	}

	splunk.ProcessEnvVars(&baseCfg)

	var cmdErr error
	switch os.Args[1] {
	case "run":
		cmdErr = runCmd(os.Args[2:], baseCfg)
	case "start":
		cmdErr = startCmd(os.Args[2:], baseCfg)
	case "status":
		cmdErr = statusCmd(os.Args[2:], baseCfg)
	case "results":
		cmdErr = resultsCmd(os.Args[2:], baseCfg)
	case "help":
		printHelp(os.Args[2:])
	case "--help", "-h":
		printUsage()
	default:
		if len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "-") {
			printUsage()
			cmdErr = errors.New("a command (run, start, etc.) is required before flags")
		} else {
			cmdErr = fmt.Errorf("unknown command: %s", os.Args[1])
		}
	}

	if cmdErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v", cmdErr)
		os.Exit(1)
	}
}
