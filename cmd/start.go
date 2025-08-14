package cmd

import (
	"errors"
	"flag"
	"fmt"

	"splunk_cli/splunk"
)

func startCmd(args []string, baseCfg splunk.Config) error {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	spl := fs.String("spl", "", "SPL query to execute")
	file := fs.String("file", "", "Read SPL query from a file (use '-' for stdin)")
	fs.StringVar(file, "f", "", "Shorthand for --file")
	earliest := fs.String("earliest", "", "Search earliest time (e.g., -1h, @d, 1672531200)")
	latest := fs.String("latest", "", "Search latest time (e.g., now, @d, 1672617600)")
	silent := fs.Bool("silent", true, "Suppress progress messages")
	addCommonFlags(fs, &baseCfg)
	fs.Parse(args)

	finalSpl, err := getSplQuery(*spl, *file)
	if err != nil {
		return err
	}
	if baseCfg.Host == "" {
		return errors.New("--host is required")
	}
	if err := promptForCredentials(&baseCfg); err != nil {
		return err
	}

	client, err := splunk.NewClient(&baseCfg, *silent)
	if err != nil {
		return err
	}
	if baseCfg.Debug {
		printDebugConfig(&baseCfg, client.Log)
	}

	client.Log.Println("Connecting to Splunk and starting search job...")
	sid, err := client.StartSearch(finalSpl, *earliest, *latest)
	if err != nil {
		return err
	}
	fmt.Println(sid)
	return nil
}
