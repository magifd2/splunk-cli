package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"splunk_cli/splunk"

	"golang.org/x/term"
)

// addCommonFlags defines flags common to all subcommands.
func addCommonFlags(fs *flag.FlagSet, cfg *splunk.Config) {
	fs.StringVar(&cfg.Host, "host", cfg.Host, "Splunk server URL (or use SPLUNK_HOST env var)")
	fs.StringVar(&cfg.Token, "token", cfg.Token, "Splunk authentication token (or use SPLUNK_TOKEN env var)")
	fs.StringVar(&cfg.User, "user", cfg.User, "Splunk username (or use SPLUNK_USER env var)")
	fs.StringVar(&cfg.Password, "password", cfg.Password, "Splunk password (or use SPLUNK_PASSWORD env var)")
	fs.StringVar(&cfg.App, "app", cfg.App, "App context for the search (or use SPLUNK_APP env var)")
	fs.BoolVar(&cfg.Insecure, "insecure", cfg.Insecure, "Skip TLS certificate verification")
	fs.DurationVar(&cfg.HTTPTimeout, "http-timeout", cfg.HTTPTimeout, "Timeout for individual HTTP requests (e.g., '5s', '1m')")
	fs.BoolVar(&cfg.Debug, "debug", false, "Enable verbose debug logging")
}

// getChoiceFromTTY reads a single line of input from the terminal, bypassing stdin.
func getChoiceFromTTY() string {
	var reader *bufio.Reader
	if runtime.GOOS == "windows" {
		reader = bufio.NewReader(os.Stdin)
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open /dev/tty, falling back to stdin: %v", err)
			reader = bufio.NewReader(os.Stdin)
		} else {
			defer tty.Close()
			reader = bufio.NewReader(tty)
		}
	}
	choice, _ := reader.ReadString('\n')
	return strings.TrimSpace(choice)
}

func printDebugConfig(cfg *splunk.Config, log *splunk.Logger) {
	maskedToken := ""
	if len(cfg.Token) > 8 {
		maskedToken = "toke..." + cfg.Token[len(cfg.Token)-4:]
	}
	maskedPassword := ""
	if cfg.Password != "" {
		maskedPassword = "********"
	}
	log.Debugf("Final configuration:")
	log.Debugf("  Host: %s", cfg.Host)
	log.Debugf("  Token: %s", maskedToken)
	log.Debugf("  User: %s", cfg.User)
	log.Debugf("  Password: %s", maskedPassword)
	log.Debugf("  App: %s", cfg.App)
	log.Debugf("  Insecure: %t", cfg.Insecure)
	log.Debugf("  HTTP Timeout: %s", cfg.HTTPTimeout)
}

func promptForCredentials(cfg *splunk.Config) error {
	if cfg.Token != "" || (cfg.User != "" && cfg.Password != "") {
		return nil
	}

	if cfg.User == "" {
		fmt.Fprintln(os.Stderr, "Authentication credentials were not provided.")
		fmt.Fprint(os.Stderr, "Enter Splunk authentication token: ")
		byteToken, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("could not read token: %w", err)
		}
		cfg.Token = string(byteToken)
		fmt.Fprintln(os.Stderr)
	} else if cfg.Password == "" {
		fmt.Fprintf(os.Stderr, "Enter Splunk password for '%s': ", cfg.User)
		bytePass, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("could not read password: %w", err)
		}
		cfg.Password = string(bytePass)
		fmt.Fprintln(os.Stderr)
	}
	return nil
}

// getSplQuery determines the SPL query from either the --spl flag or --file flag.
func getSplQuery(splFlag, fileFlag string) (string, error) {
	if splFlag != "" && fileFlag != "" {
		return "", errors.New("--spl and --file flags cannot be used at the same time")
	}
	if splFlag != "" {
		return splFlag, nil
	}
	if fileFlag != "" {
		var splBytes []byte
		var err error
		if fileFlag == "-" {
			splBytes, err = io.ReadAll(os.Stdin)
		} else {
			splBytes, err = os.ReadFile(fileFlag)
		}
		if err != nil {
			return "", fmt.Errorf("failed to read SPL from file '%s': %w", fileFlag, err)
		}
		return string(splBytes), nil
	}
	return "", errors.New("--spl or --file flag is required")
}

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

	done, jobState, _, err := client.JobStatus(*sid)
	if err != nil {
		return err
	}
	fmt.Printf("SID: %s\nIsDone: %t\nDispatchState: %s", *sid, done, jobState)
	return nil
}

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
	results, err := client.Results(*sid)
	if err != nil {
		return err
	}
	fmt.Println(results)
	return nil
}

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

// These variables are set by the linker.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var showVersion bool
	var configPath string // New variable for custom config path

	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.StringVar(&configPath, "config", "", "Path to a custom configuration file") // New flag
	flag.Parse()

	if showVersion {
		fmt.Printf("splunk-cli version %s\ncommit %s\nbuilt at %s", version, commit, date)
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// A simple logger for startup errors
	log := &splunk.Logger{}
	baseCfg, cfgPath, err := splunk.LoadConfigFromFile(configPath) // Pass configPath here
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