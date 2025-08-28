package splunk

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client holds the state for a command execution, including the HTTP client.
type Client struct {
	client *http.Client
	cfg    *Config
	Log    *Logger
}

// Logger provides a simple logger that can be silenced.
type Logger struct {
	silent bool
	debug  bool
}

func (l *Logger) Printf(format string, a ...any) {
	if !l.silent {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func (l *Logger) Println(a ...any) {
	if !l.silent {
		fmt.Fprintln(os.Stderr, a...)
	}
}

func (l *Logger) Debugf(format string, a ...any) {
	if l.debug {
		fmt.Fprintf(os.Stderr, "DEBUG: "+format, a...)
	}
}

// NewClient creates a new state object, including the HTTP client with a proper cookie jar.
func NewClient(cfg *Config, silent bool) (*Client, error) {
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

	return &Client{
		client: client,
		cfg:    cfg,
		Log:    &Logger{silent: silent && !cfg.Debug, debug: cfg.Debug},
	}, nil
}

func (c *Client) createAPIURL(pathSegments ...string) (string, error) {
	baseURL, err := url.Parse(c.cfg.Host)
	if err != nil {
		return "", fmt.Errorf("invalid host URL in configuration: %w", err)
	}

	var finalSegments []string
	if c.cfg.App != "" {
		owner := c.cfg.Owner
		if owner == "" {
			owner = "nobody"
		}
		finalSegments = append([]string{"servicesNS", owner, c.cfg.App}, pathSegments...)
	} else {
		finalSegments = append([]string{"services"}, pathSegments...)
	}

	fullURL := baseURL.JoinPath(finalSegments...)
	return fullURL.String(), nil
}

func (c *Client) handleFailedResponse(resp *http.Response, expectedStatus int) error {
	if resp.StatusCode == expectedStatus {
		return nil
	}

	if c.Log.debug {
		c.Log.Debugf(`Response Headers:
`)
		for k, v := range resp.Header {
			c.Log.Debugf(`  %s: %s
`, k, strings.Join(v, ", "))
		}
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf(`API request failed with status %s. Response: %s`, resp.Status, string(body))
}

func (c *Client) setupAuth(req *http.Request) error {
	if c.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	} else if c.cfg.User != "" && c.cfg.Password != "" {
		req.SetBasicAuth(c.cfg.User, c.cfg.Password)
	}
	return nil
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	if err := c.setupAuth(req); err != nil {
		return nil, err
	}

	if c.Log.debug {
		dump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			c.Log.Debugf(`Error dumping request: %v
`, err)
		} else {
			dumpStr := string(dump)
			if c.cfg.Token != "" {
				dumpStr = strings.Replace(dumpStr, c.cfg.Token, "<TOKEN>", 1)
			}
			c.Log.Debugf(
				`
--- BEGIN HTTP REQUEST DUMP ---
%s
--- END HTTP REQUEST DUMP ---
`,
				dumpStr,
			)
		}
	}

	return c.client.Do(req)
}

// StartSearch initiates a search job on Splunk.
func (c *Client) StartSearch(spl, earliest, latest string) (string, error) {
	endpoint, err := c.createAPIURL("search", "jobs")
	if err != nil {
		return "", err
	}
	c.Log.Debugf(`Request: POST %s
`, endpoint)

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

	resp, err := c.doRequest(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := c.handleFailedResponse(resp, http.StatusCreated); err != nil {
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

type SplunkMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// JobStatus retrieves the current status of a search job.
func (c *Client) JobStatus(sid string) (bool, string, []SplunkMessage, int, error) {
	endpoint, err := c.createAPIURL("search", "jobs", sid)
	if err != nil {
		return false, "", nil, 0, err
	}
	c.Log.Debugf(`Request: GET %s
`, endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return false, "", nil, 0, err
	}

	q := req.URL.Query()
	q.Add("output_mode", "json")
	req.URL.RawQuery = q.Encode()

	resp, err := c.doRequest(req)
	if err != nil {
		return false, "", nil, 0, err
	}
	defer resp.Body.Close()

	if err := c.handleFailedResponse(resp, http.StatusOK); err != nil {
		return false, "", nil, 0, err
	}

	var status struct {
		Entry []struct {
			Content struct {
				IsDone        bool            `json:"isDone"`
				DispatchState string          `json:"dispatchState"`
				Messages      []SplunkMessage `json:"messages"`
				ResultCount   int             `json:"resultCount"`
			} `json:"content"`
		} `json:"entry"`
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", nil, 0, fmt.Errorf(`failed to read job status response body: %w`, err)
	}

	if err := json.Unmarshal(bodyBytes, &status); err != nil {
		return false, "", nil, 0, fmt.Errorf(`failed to decode job status JSON: %w. Received: %s`, err, string(bodyBytes))
	}

	if len(status.Entry) == 0 {
		return false, "", nil, 0, errors.New("job status not found in response")
	}
	content := status.Entry[0].Content
	return content.IsDone, content.DispatchState, content.Messages, content.ResultCount, nil
}


// WaitForJob waits for a job to finish, with a timeout.
func (c *Client) WaitForJob(ctx context.Context, sid string) error {
	c.Log.Println("Waiting for job to complete...")
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			done, jobState, messages, _, err := c.JobStatus(sid)
			if err != nil {
				return err
			}

			if done {
				if jobState == "FAILED" {
					var errorMessages strings.Builder
					for _, msg := range messages {
						if strings.ToUpper(msg.Type) == "FATAL" || strings.ToUpper(msg.Type) == "ERROR" {
							errorMessages.WriteString(fmt.Sprintf(`
  - %s`, msg.Text))
						}
					}
					if errorMessages.Len() > 0 {
						return fmt.Errorf(`search job %s failed with errors:%s`, sid, errorMessages.String())
					}
					return fmt.Errorf(`search job %s failed`, sid)
				}
				c.Log.Println("Job finished.")
				return nil
			}
		}
	}
}

// Results fetches the results of a completed search job, handling pagination.
func (c *Client) Results(sid string, limit int) (string, error) {
	// 1. Get the total number of results for the job
	_, _, _, totalResults, err := c.JobStatus(sid)
	if err != nil {
		return "", fmt.Errorf("could not get job status before fetching results: %w", err)
	}

	// 2. Determine the number of results to fetch
	fetchCount := limit
	if limit == 0 || (limit > 0 && limit > totalResults) {
		fetchCount = totalResults
	}

	// 3. Fetch results, with pagination if necessary
	const maxCount = 50000 // Max results per request
	var allResults []json.RawMessage

	for offset := 0; offset < fetchCount; offset += maxCount {
		// Determine count for this specific request
		count := maxCount
		if offset+count > fetchCount {
			count = fetchCount - offset
		}

		// Prepare request
		endpoint, err := c.createAPIURL("search", "jobs", sid, "results")
		if err != nil {
			return "", err
		}
		c.Log.Debugf(`Request: GET %s (offset: %d, count: %d)
`, endpoint, offset, count)

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return "", err
		}
		q := req.URL.Query()
		q.Add("output_mode", "json")
		q.Add("offset", fmt.Sprintf("%d", offset))
		q.Add("count", fmt.Sprintf("%d", count))
		req.URL.RawQuery = q.Encode()

		// Execute request
		resp, err := c.doRequest(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if err := c.handleFailedResponse(resp, http.StatusOK); err != nil {
			return "", err
		}

		// Decode and append results
		var page struct {
			Results []json.RawMessage `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
			return "", fmt.Errorf("failed to decode results page: %w", err)
		}
		allResults = append(allResults, page.Results...)
	}

	// 4. Combine and format the final JSON output
	finalJSON := map[string][]json.RawMessage{
		"results": allResults,
	}

	prettyJSON, err := json.MarshalIndent(finalJSON, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal final results: %w", err)
	}

	return string(prettyJSON), nil
}


// CancelSearch sends a request to cancel a running job.
func (c *Client) CancelSearch(sid string) error {
	c.Log.Println(`
Cancelling search job...`)
	endpoint, err := c.createAPIURL("search", "jobs", sid, "control")
	if err != nil {
		return err
	}
	c.Log.Debugf(`Request: POST %s
`, endpoint)

	req, err := http.NewRequest("POST", endpoint, strings.NewReader("action=cancel"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.doRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		c.Log.Println("Job successfully cancelled.")
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf(`failed to cancel job: %s, %s`, resp.Status, string(body))
}
