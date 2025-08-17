package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"splunk_cli/splunk"
)

func Execute() {
	var configPath string

	// NOTE: We are not using flag.Parse() here at the top level anymore.
	// Each command will be responsible for parsing its own flags.
	// We manually check for the config flag.
	for i, arg := range os.Args {
		if (arg == "--config" || arg == "-config") && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
			// Remove the flag and its value from os.Args so subcommands don't see it.
			os.Args = append(os.Args[:i], os.Args[i+2:]...)
			break
		}
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