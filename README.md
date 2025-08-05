# Splunk CLI Tool (splunk-cli)

**splunk-cli** is a powerful and lightweight command-line interface (CLI) tool written in Go for interacting with the Splunk REST API. It allows you to efficiently execute SPL (Search Processing Language) queries, manage search jobs, and retrieve results directly from your terminal or in scripts.

[![Lint](https://github.com/magifd2/splunk-cli/actions/workflows/lint.yml/badge.svg)](https://github.com/magifd2/splunk-cli/actions/workflows/lint.yml)
[![Test](https://github.com/magifd2/splunk-cli/actions/workflows/test.yml/badge.svg)](https://github.com/magifd2/splunk-cli/actions/workflows/test.yml)

## Features

- **Automation**: Trigger Splunk searches from shell scripts or CI/CD jobs and pipe the results into subsequent processes.
- **Efficiency**: Quickly check data with a single command without opening the Web UI.
- **Flexible Authentication**: Manage credentials via command-line flags, environment variables, a configuration file, or a secure interactive prompt.
- **Long-Running Job Management**: The asynchronous execution model (`start`, `status`, `results`) allows you to manage heavy search jobs that may take hours, without tying up your terminal.
- **App Context**: Use the `--app` flag to run searches within a specific app context, enabling the use of app-specific lookups and knowledge objects.

## Installation

There are two ways to install `splunk-cli`:

### 1. From a Release (Recommended)

You can download the pre-compiled binary for your operating system (macOS, Linux, Windows) from the [GitHub Releases page](https://github.com/magifd2/splunk-cli/releases).

### 2. From Source

If you have Go installed, you can build the tool from the source code.

```bash
# Clone the repository
git clone https://github.com/magifd2/splunk-cli.git
cd <Your-Repository-Name>

# Build the binary
make build

# The executable will be in the dist/ directory, e.g., dist/macos/splunk-cli
```

## Usage

### Configuration

The most convenient way to use the tool is by creating a configuration file.

**Path**: `~/.config/splunk-cli/config.json`

**Example Content**:
```json
{
  "host": "https://your-splunk-instance.com:8089",
  "token": "your-splunk-token-here",
  "app": "search",
  "insecure": true,
  "httpTimeout": "60s"
}
```

### Configuration Priority

Settings are evaluated in the following order of precedence (highest priority first):

1.  **Command-line Flags** (e.g., `--host <URL>`)
2.  **Environment Variables** (e.g., `SPLUNK_HOST`, `SPLUNK_APP`)
3.  **Configuration File**

### Commands

`splunk-cli` provides a set of commands for different tasks.

#### `run`

Starts a search, waits for it to complete, and displays the results.

**Examples**:
```bash
# Search data from the last hour
splunk-cli run --spl "index=_internal" --earliest "-1h"

# Read SPL from a file and execute
cat my_query.spl | splunk-cli run -f -
```

- `--spl <string>`: The SPL query to execute.
- `--file <path>` or `-f <path>`: Read the SPL query from a file. Use `-` for stdin.
- `--earliest <time>`: The earliest time for the search (e.g., -1h, @d, 1672531200).
- `--latest <time>`: The latest time for the search (e.g., now, @d, 1672617600).
- `--timeout <duration>`: Total timeout for the job (e.g., 10m, 1h30m).
- `--silent`: Suppress progress messages.

> **💡 Ctrl+C Behavior**: When you press `Ctrl+C` during a `run` command, you can choose to either cancel the job or let it continue running in the background.

#### `start`

Starts a search job and immediately prints the Job ID (SID) to stdout.

**Example**:
```bash
export JOB_ID=$(splunk-cli start --spl "index=main | stats count by sourcetype")
echo "Job started with SID: $JOB_ID"
```

#### `status`

Checks the status of a specified job SID.

**Example**:
```bash
splunk-cli status --sid "$JOB_ID"
```

#### `results`

Fetches the results of a completed job. This is useful in combination with tools like `jq`.

**Example**:
```bash
splunk-cli results --sid "$JOB_ID" --silent | jq .
```

### Common Flags

These flags are available for most commands:

- `--host <url>`: The URL of the Splunk server.
- `--token <string>`: The authentication token.
- `--user <string>`: The username.
- `--password <string>`: The password (will be prompted for if not provided).
- `--app <string>`: The app context for the search.
- `--owner <string>`: The owner of knowledge objects within the app (defaults to `nobody`).
- `--insecure`: Skip TLS certificate verification.
- `--http-timeout <duration>`: Timeout for individual API requests (e.g., 30s, 1m).
- `--debug`: Enable detailed debug logging.
- `--version`: Print version information.

## Development

This project uses a `Makefile` for common development tasks.

- `make build`: Build binaries for all target platforms (macOS, Linux, Windows).
- `make test`: Run tests.
- `make lint`: Run the linter (`golangci-lint`).
- `make vulncheck`: Scan for known vulnerabilities (`govulncheck`).
- `make clean`: Clean up build artifacts.

## License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) file for details.

---

*This tool was bootstrapped and developed in collaboration with Gemini, a large language model from Google.*
