package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// config stores all configuration options.
type config struct {
	Host        string        `json:"host"`
	Token       string        `json:"token"`
	User        string        `json:"user"`
	Password    string        `json:"password"`
	App         string        `json:"app"`
	Owner       string        `json:"owner"`
	Insecure    bool          `json:"insecure"`
	HTTPTimeout time.Duration `json:"httpTimeout"`
	Debug       bool          `json:"-"` // Exclude from JSON marshalling
}

// clientState holds the state for a command execution, including the HTTP client.
type clientState struct {
	client *http.Client
	cfg    *config
	log    *logger
}

// logger provides a simple logger that can be silenced.
type logger struct {
	silent bool
	debug  bool
}

func (l *logger) Printf(format string, a ...any) {
	if !l.silent {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func (l *logger) Println(a ...any) {
	if !l.silent {
		fmt.Fprintln(os.Stderr, a...)
	}
}

func (l *logger) Debugf(format string, a ...any) {
	if l.debug {
		fmt.Fprintf(os.Stderr, "DEBUG: "+format, a...)
	}
}

// loadConfigFromFile loads configuration from the user's config directory.
func loadConfigFromFile() (config, string, error) {
	var cfg config
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, "", fmt.Errorf("could not get user home directory: %w", err)
	}

	configFile := filepath.Join(home, ".config", "splunk-cli", "config.json")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return cfg, configFile, nil
	}

	file, err := os.Open(configFile)
	if err != nil {
		return cfg, configFile, fmt.Errorf("could not open config file: %w", err)
	}
	defer file.Close()

	type configHelper struct {
		Host        string `json:"host"`
		Token       string `json:"token"`
		User        string `json:"user"`
		Password    string `json:"password"`
		App         string `json:"app"`
		Owner       string `json:"owner"`
		Insecure    bool   `json:"insecure"`
		HTTPTimeout string `json:"httpTimeout"`
	}
	var helper configHelper
	if err := json.NewDecoder(file).Decode(&helper); err != nil {
		return cfg, configFile, fmt.Errorf("could not parse config file: %w", err)
	}

	cfg.Host = strings.TrimSpace(helper.Host)
	cfg.Token = strings.TrimSpace(helper.Token)
	cfg.User = strings.TrimSpace(helper.User)
	cfg.Password = strings.TrimSpace(helper.Password)
	cfg.App = strings.TrimSpace(helper.App)
	cfg.Owner = strings.TrimSpace(helper.Owner)
	cfg.Insecure = helper.Insecure
	if helper.HTTPTimeout != "" {
		parsedDuration, err := time.ParseDuration(helper.HTTPTimeout)
		if err != nil {
			return cfg, configFile, fmt.Errorf("invalid httpTimeout value in config: %w", err)
		}
		cfg.HTTPTimeout = parsedDuration
	}

	return cfg, configFile, nil
}

// createAPIURL creates a full URL from the base host and path segments.
func createAPIURL(cfg *config, pathSegments ...string) (string, error) {
	baseURL, err := url.Parse(cfg.Host)
	if err != nil {
		return "", fmt.Errorf("invalid host URL in configuration: %w", err)
	}

	var finalSegments []string
	if cfg.App != "" {
		owner := cfg.Owner
		if owner == "" {
			owner = "nobody"
		}
		finalSegments = append([]string{"servicesNS", owner, cfg.App}, pathSegments...)
	} else {
		finalSegments = append([]string{"services"}, pathSegments...)
	}

	fullURL := baseURL.JoinPath(finalSegments...)
	return fullURL.String(), nil
}

// newClientState creates a new state object, including the HTTP client with a proper cookie jar.
func newClientState(cfg *config, silent bool) (*clientState, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("fatal: could not create cookie jar: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: cfg.Insecure}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.HTTPTimeout,
		Jar:       jar,
	}

	return &clientState{
		client: client,
		cfg:    cfg,
		log:    &logger{silent: silent && !cfg.Debug, debug: cfg.Debug},
	}, nil
}

// handleFailedResponse checks for non-successful HTTP responses.
func handleFailedResponse(resp *http.Response, expectedStatus int, state *clientState) error {
	if resp.StatusCode == expectedStatus {
		return nil
	}

	if state.log.debug {
		state.log.Debugf("Response Headers:\n")
		for k, v := range resp.Header {
			state.log.Debugf("  %s: %s\n", k, strings.Join(v, ", "))
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("API request failed with status %s. Response: %s", resp.Status, string(body))
}

// setupAuth configures authentication for an HTTP request.
func setupAuth(req *http.Request, state *clientState) error {
	if state.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+state.cfg.Token)
	} else if state.cfg.User != "" && state.cfg.Password != "" {
		req.SetBasicAuth(state.cfg.User, state.cfg.Password)
	}
	return nil
}

// doRequest executes an HTTP request after setting auth and handling debug logging.
func doRequest(req *http.Request, state *clientState) (*http.Response, error) {
	if err := setupAuth(req, state); err != nil {
		return nil, err
	}

	if state.log.debug {
		dump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			state.log.Debugf("Error dumping request: %v\n", err)
		} else {
			dumpStr := string(dump)
			if state.cfg.Token != "" {
				dumpStr = strings.Replace(dumpStr, state.cfg.Token, "<TOKEN>", 1)
			}
			state.log.Debugf("\n--- BEGIN HTTP REQUEST DUMP ---\n%s\n--- END HTTP REQUEST DUMP ---\n", dumpStr)
		}
	}

	return state.client.Do(req)
}

// startSearchJob initiates a search job on Splunk.
func startSearchJob(spl, earliest, latest string, state *clientState) (string, error) {
	endpoint, err := createAPIURL(state.cfg, "search", "jobs")
	if err != nil {
		return "", err
	}
	state.log.Debugf("Request: POST %s\n", endpoint)

	form := url.Values{}
	if !strings.HasPrefix(strings.TrimSpace(spl), "|") {
		form.Set("search", "search "+spl)
	} else {
		form.Set("search", spl)
	}
	if earliest != "" {
		form.Set("earliest_time", earliest)
	}
	if latest != "" {
		form.Set("latest_time", latest)
	}
	form.Set("output_mode", "json")

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := doRequest(req, state)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := handleFailedResponse(resp, http.StatusCreated, state); err != nil {
		return "", err
	}

	var job struct {
		SID string `json:"sid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return "", err
	}
	return job.SID, nil
}

type splunkMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// getJobStatus retrieves the current status of a search job.
func getJobStatus(sid string, state *clientState) (bool, string, []splunkMessage, error) {
	endpoint, err := createAPIURL(state.cfg, "search", "jobs", sid)
	if err != nil {
		return false, "", nil, err
	}
	state.log.Debugf("Request: GET %s\n", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return false, "", nil, err
	}
	
	q := req.URL.Query()
	q.Add("output_mode", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := doRequest(req, state)
	if err != nil {
		return false, "", nil, err
	}
	defer resp.Body.Close()

	if err := handleFailedResponse(resp, http.StatusOK, state); err != nil {
		return false, "", nil, err
	}

	var status struct {
		Entry []struct {
			Content struct {
				IsDone        bool            `json:"isDone"`
				DispatchState string          `json:"dispatchState"`
				Messages      []splunkMessage `json:"messages"`
			} `json:"content"`
		} `json:"entry"`
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", nil, fmt.Errorf("failed to read job status response body: %w", err)
	}

	if err := json.Unmarshal(bodyBytes, &status); err != nil {
		return false, "", nil, fmt.Errorf("failed to decode job status JSON: %w. Received: %s", err, string(bodyBytes))
	}

	if len(status.Entry) == 0 {
		return false, "", nil, errors.New("job status not found in response")
	}
	content := status.Entry[0].Content
	return content.IsDone, content.DispatchState, content.Messages, nil
}

// waitForJobCompletion waits for a job to finish, with a timeout.
func waitForJobCompletion(ctx context.Context, sid string, state *clientState) error {
	state.log.Println("Waiting for job to complete...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			done, jobState, messages, err := getJobStatus(sid, state)
			if err != nil {
				return err
			}

			if done {
				if jobState == "FAILED" {
					var errorMessages strings.Builder
					for _, msg := range messages {
						if strings.ToUpper(msg.Type) == "FATAL" || strings.ToUpper(msg.Type) == "ERROR" {
							errorMessages.WriteString(fmt.Sprintf("\n  - %s", msg.Text))
						}
					}
					if errorMessages.Len() > 0 {
						return fmt.Errorf("search job %s failed with errors:%s", sid, errorMessages.String())
					}
					return fmt.Errorf("search job %s failed", sid)
				}
				state.log.Println("Job finished.")
				return nil
			}
		}
	}
}

// getJobResults fetches the results of a completed search job.
func getJobResults(sid string, state *clientState) (string, error) {
	endpoint, err := createAPIURL(state.cfg, "search", "jobs", sid, "results")
	if err != nil {
		return "", err
	}
	state.log.Debugf("Request: GET %s\n", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Add("output_mode", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := doRequest(req, state)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := handleFailedResponse(resp, http.StatusOK, state); err != nil {
		return "", err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err != nil {
		return string(body), nil
	}
	return prettyJSON.String(), nil
}

// cancelSearchJob sends a request to cancel a running job.
func cancelSearchJob(sid string, state *clientState) error {
	state.log.Println("\nCancelling search job...")
	endpoint, err := createAPIURL(state.cfg, "search", "jobs", sid, "control")
	if err != nil {
		return err
	}
	state.log.Debugf("Request: POST %s\n", endpoint)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader("action=cancel"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := doRequest(req, state)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		state.log.Println("Job successfully cancelled.")
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to cancel job: %s, %s", resp.Status, string(body))
}

// processEnvVars overwrites config with values from environment variables.
func processEnvVars(cfg *config) {
	if host := os.Getenv("SPLUNK_HOST"); host != "" {
		cfg.Host = host
	}
	if token := os.Getenv("SPLUNK_TOKEN"); token != "" {
		cfg.Token = token
	}
	if user := os.Getenv("SPLUNK_USER"); user != "" {
		cfg.User = user
	}
	if password := os.Getenv("SPLUNK_PASSWORD"); password != "" {
		cfg.Password = password
	}
	if app := os.Getenv("SPLUNK_APP"); app != "" {
		cfg.App = app
	}
}

// addCommonFlags defines flags common to all subcommands.
func addCommonFlags(fs *flag.FlagSet, cfg *config) {
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
			fmt.Fprintf(os.Stderr, "Warning: could not open /dev/tty, falling back to stdin: %v\n", err)
			reader = bufio.NewReader(os.Stdin)
		} else {
			defer tty.Close()
			reader = bufio.NewReader(tty)
		}
	}
	choice, _ := reader.ReadString('\n')
	return strings.TrimSpace(choice)
}

func printDebugConfig(cfg *config, log *logger) {
	maskedToken := ""
	if len(cfg.Token) > 8 {
		maskedToken = "toke..." + cfg.Token[len(cfg.Token)-4:]
	}
	maskedPassword := ""
	if cfg.Password != "" {
		maskedPassword = "********"
	}
	log.Debugf("Final configuration:\n")
	log.Debugf("  Host: %s\n", cfg.Host)
	log.Debugf("  Token: %s\n", maskedToken)
	log.Debugf("  User: %s\n", cfg.User)
	log.Debugf("  Password: %s\n", maskedPassword)
	log.Debugf("  App: %s\n", cfg.App)
	log.Debugf("  Insecure: %t\n", cfg.Insecure)
	log.Debugf("  HTTP Timeout: %s\n", cfg.HTTPTimeout)
}

func promptForCredentials(cfg *config) error {
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

func runCmd(args []string, baseCfg config) error {
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

	state, err := newClientState(&baseCfg, *silent)
	if err != nil {
		return err
	}
	if state.log.debug {
		printDebugConfig(&baseCfg, state.log)
	}

	state.log.Println("Connecting to Splunk and starting search job...")
	sid, err := startSearchJob(finalSpl, *earliest, *latest, state)
	if err != nil {
		return err
	}
	state.log.Printf("Job started with SID: %s\n", sid)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- waitForJobCompletion(ctx, sid, state)
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
				fmt.Fprintf(os.Stderr, "Detaching from job %s. Use 'results' command to fetch results later.\n", sid)
				return nil
			}
		case <-secondSigChan:
		}
		return cancelSearchJob(sid, state)
	}

	state.log.Println("Fetching results...")
	results, err := getJobResults(sid, state)
	if err != nil {
		return err
	}
	fmt.Println(results)
	return nil
}

func startCmd(args []string, baseCfg config) error {
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

	state, err := newClientState(&baseCfg, *silent)
	if err != nil {
		return err
	}
	if state.log.debug {
		printDebugConfig(&baseCfg, state.log)
	}

	state.log.Println("Connecting to Splunk and starting search job...")
	sid, err := startSearchJob(finalSpl, *earliest, *latest, state)
	if err != nil {
		return err
	}
	fmt.Println(sid)
	return nil
}

func statusCmd(args []string, baseCfg config) error {
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

	state, err := newClientState(&baseCfg, false)
	if err != nil {
		return err
	}
	if state.log.debug {
		printDebugConfig(&baseCfg, state.log)
	}

	done, jobState, _, err := getJobStatus(*sid, state)
	if err != nil {
		return err
	}
	fmt.Printf("SID: %s\nIsDone: %t\nDispatchState: %s\n", *sid, done, jobState)
	return nil
}

func resultsCmd(args []string, baseCfg config) error {
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

	state, err := newClientState(&baseCfg, *silent)
	if err != nil {
		return err
	}
	if state.log.debug {
		printDebugConfig(&baseCfg, state.log)
	}

	done, jobState, _, err := getJobStatus(*sid, state)
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("job %s is not complete yet (state: %s)", *sid, jobState)
	}
	if jobState == "FAILED" {
		return fmt.Errorf("cannot get results, job %s failed", *sid)
	}

	state.log.Println("Fetching results...")
	results, err := getJobResults(*sid, state)
	if err != nil {
		return err
	}
	fmt.Println(results)
	return nil
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: splunk-cli <command> [options]")
	fmt.Fprintln(os.Stderr, "\nA flexible CLI tool to interact with the Splunk REST API.")
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
	dummyCfg := config{}

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
		fmt.Fprintf(os.Stderr, "Error: Unknown command for help: %s\n", cmd)
		return
	}
	addCommonFlags(fs, &dummyCfg)
	fmt.Fprintf(os.Stderr, "Usage: splunk-cli %s [options]\n\nOptions for %s:\n", cmd, cmd)
	fs.PrintDefaults()
}



// These variables are set by the linker.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "Print version information and exit")
	flag.Parse()

	if showVersion {
		fmt.Printf("splunk-cli version %s\ncommit %s\nbuilt at %s\n", version, commit, date)
		os.Exit(0)
	}


	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	log := &logger{}
	baseCfg, cfgPath, err := loadConfigFromFile()
	if err != nil {
		log.Printf("Warning: could not load config file at %s: %v\n", cfgPath, err)
	}

	if baseCfg.HTTPTimeout == 0 {
		baseCfg.HTTPTimeout = 30 * time.Second
	}

	processEnvVars(&baseCfg)

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
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmdErr)
		os.Exit(1)
	}
}
