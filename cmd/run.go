package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"splunk_cli/splunk"
)

func runCmd(args []string, baseCfg splunk.Config) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	spl := fs.String("spl", "", "SPL query to execute")
	file := fs.String("file", "", "Read SPL query from a file (use '-' for stdin)")
	fs.StringVar(file, "f", "", "Shorthand for --file")
	earliest := fs.String("earliest", "", "Search earliest time (e.g., -1h, @d, 1672531200)")
	latest := fs.String("latest", "", "Search latest time (e.g., now, @d, 1672617600)")
	timeout := fs.Duration("timeout", 10*time.Minute, "Total timeout for the run command")
	silent := fs.Bool("silent", false, "Suppress progress messages")
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
	client.Log.Printf("Job started with SID: %s", sid)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- client.WaitForJob(ctx, sid)
	}()

	select {
	case err := <-errChan:
		if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("command timed out after %v", *timeout)
		}
	case <-sigChan:
		signal.Stop(sigChan)
		fmt.Fprintf(os.Stderr, "\n^C detected. What would you like to do?\n  (c)ancel the job on Splunk\n  (d)etach and let it run in the background\nChoice [c/d]: ")

		choiceChan := make(chan string)
		go func() {
			choiceChan <- getChoiceFromTTY()
		}()

		secondSigChan := make(chan os.Signal, 1)
		signal.Notify(secondSigChan, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(secondSigChan)

		select {
		case choice := <-choiceChan:
			if strings.ToLower(choice) == "d" {
				fmt.Fprintf(os.Stderr, "Detaching from job %s. Use 'results' command to fetch results later.", sid)
				return nil
			}
		case <-secondSigChan:
		}
		return client.CancelSearch(sid)
	}

	client.Log.Println("Fetching results...")
	results, err := client.Results(sid)
	if err != nil {
		return err
	}
	fmt.Println(results)
	return nil
}
