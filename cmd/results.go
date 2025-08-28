package cmd

import (
	"errors"
	"flag"
	"fmt"

	"splunk_cli/splunk"
)

func resultsCmd(args []string, baseCfg splunk.Config) error {
	fs := flag.NewFlagSet("results", flag.ExitOnError)
	sid := fs.String("sid", "", "Search ID (SID) of the job")
	silent := fs.Bool("silent", false, "Suppress progress messages")
	addCommonFlags(fs, &baseCfg)
	fs.Parse(args)

	if *sid == "" {
		return errors.New("--sid is a required argument for 'results'")
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

	done, jobState, _, err := client.JobStatus(*sid)
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("job %s is not complete yet (state: %s)", *sid, jobState)
	}
	if jobState == "FAILED" {
		return fmt.Errorf("cannot get results, job %s failed", *sid)
	}

	client.Log.Println("Fetching results...")
	results, err := client.Results(*sid, baseCfg.Limit)
	if err != nil {
		return err
	}
	fmt.Println(results)
	return nil
}
