package cmd

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"syscall"

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
	fs.IntVar(&cfg.Limit, "limit", cfg.Limit, "Maximum number of results to return (0 for all)")
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
