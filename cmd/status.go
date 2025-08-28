package cmd

import (
	"errors"
	"flag"
	"fmt"

	"splunk_cli/splunk"
)

func statusCmd(args []string, baseCfg splunk.Config) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	sid := fs.String("sid", "", "Search ID (SID) of the job")
	addCommonFlags(fs, &baseCfg)
	fs.Parse(args)

	if *sid == "" {
		return errors.New("--sid is a required argument for 'status'")
	}
	if baseCfg.Host == "" {
		return errors.New("--host is required")
	}
	if err := promptForCredentials(&baseCfg); err != nil {
		return err
	}

	client, err := splunk.NewClient(&baseCfg, false)
	if err != nil {
		return err
	}
	if baseCfg.Debug {
		printDebugConfig(&baseCfg, client.Log)
	}

	done, jobState, _, _, err := client.JobStatus(*sid)
	if err != nil {
		return err
	}
	fmt.Printf("SID: %s\nIsDone: %t\nDispatchState: %s", *sid, done, jobState)
	return nil
}
