package main

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"
const epubMetadataVersion = "3"

type Config struct {
	MatterToken         string
	APIBaseURL          string
	OutputDir           string
	StatePath           string
	LogPath             string
	CACertFile          string
	SyncStatus          string
	ContentTypes        []string
	MaxItemsPerSync     int
	AutoSyncEnabled     bool
	SyncIntervalMinutes int
	NetworkWaitSeconds  int
	AutoConnectWiFi     bool
	TriggerKoboRescan   bool
}

type State struct {
	LastSyncStartedAt  string               `json:"last_sync_started_at"`
	LastSyncFinishedAt string               `json:"last_sync_finished_at"`
	LastError          string               `json:"last_error,omitempty"`
	Items              map[string]ItemState `json:"items"`
}

type ItemState struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	UpdatedAt           string `json:"updated_at"`
	FilePath            string `json:"file_path"`
	EPUBMetadataVersion string `json:"epub_metadata_version,omitempty"`
}

type MatterClient struct {
	baseURL string
	token   string
	http    *http.Client
}

type listResponse struct {
	Object     string       `json:"object"`
	Results    []MatterItem `json:"results"`
	HasMore    bool         `json:"has_more"`
	NextCursor string       `json:"next_cursor"`
}

type matterAuthor struct {
	Name string `json:"name"`
}

type matterTag struct {
	Name string `json:"name"`
}

type MatterItem struct {
	Object           string       `json:"object"`
	ID               string       `json:"id"`
	Title            string       `json:"title"`
	URL              string       `json:"url"`
	SiteName         string       `json:"site_name"`
	Author           matterAuthor `json:"author"`
	Status           string       `json:"status"`
	ProcessingStatus string       `json:"processing_status"`
	ContentType      string       `json:"content_type"`
	WordCount        *int         `json:"word_count"`
	ReadingProgress  float64      `json:"reading_progress"`
	Markdown         string       `json:"markdown"`
	Excerpt          string       `json:"excerpt"`
	LibraryPosition  *int64       `json:"library_position"`
	InboxPosition    *int64       `json:"inbox_position"`
	Tags             []matterTag  `json:"tags"`
	UpdatedAt        string       `json:"updated_at"`
}

type accountResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	IsPro bool   `json:"is_pro"`
}

func defaultConfig() Config {
	return Config{
		APIBaseURL:          "https://api.getmatter.com/public/v1",
		OutputDir:           "/mnt/onboard/Matter",
		StatePath:           "/mnt/onboard/.adds/matter/state.json",
		LogPath:             "/mnt/onboard/.adds/matter/matter.log",
		CACertFile:          "/mnt/onboard/.adds/matter/cacert.pem",
		SyncStatus:          "queue",
		ContentTypes:        []string{"article", "newsletter"},
		MaxItemsPerSync:     50,
		AutoSyncEnabled:     true,
		SyncIntervalMinutes: 60,
		NetworkWaitSeconds:  180,
		AutoConnectWiFi:     true,
		TriggerKoboRescan:   true,
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	if len(os.Args) < 2 {
		runCLI("once", os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "once", "daemon", "status", "test", "version":
		runCLI(os.Args[1], os.Args[2:])
	default:
		runCLI("once", os.Args[1:])
	}
}

func runCLI(command string, args []string) {
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	configPath := fs.String("config", "/mnt/onboard/.adds/matter/config.env", "path to config.env")
	if err := fs.Parse(args); err != nil {
		die(err)
	}

	if command == "version" {
		fmt.Println(version)
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		die(err)
	}
	closeLog, err := attachLog(cfg.LogPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open log: %v\n", err)
	} else {
		defer closeLog()
	}

	switch command {
	case "once":
		err = runOnce(context.Background(), cfg)
	case "daemon":
		err = runDaemon(context.Background(), *configPath, cfg)
	case "status":
		err = printStatus(cfg)
	case "test":
		err = testMatter(context.Background(), cfg)
	default:
		err = fmt.Errorf("unknown command: %s", command)
	}
	if err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func attachLog(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	return func() { _ = f.Close() }, nil
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		sample := path + ".sample"
		if _, sampleErr := os.Stat(sample); sampleErr == nil {
			return cfg, fmt.Errorf("config missing: copy %s to %s and set MATTER_TOKEN", sample, path)
		}
		return cfg, fmt.Errorf("config missing: %s", path)
	}

	lines := strings.Split(string(data), "\n")
	for idx, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return cfg, fmt.Errorf("%s:%d: expected KEY=value", path, idx+1)
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"`)
		switch key {
		case "MATTER_TOKEN":
			cfg.MatterToken = val
		case "API_BASE_URL":
			cfg.APIBaseURL = strings.TrimRight(val, "/")
		case "OUTPUT_DIR":
			cfg.OutputDir = val
		case "STATE_PATH":
			cfg.StatePath = val
		case "LOG_PATH":
			cfg.LogPath = val
		case "CA_CERT_FILE":
			cfg.CACertFile = val
		case "SYNC_STATUS":
			cfg.SyncStatus = val
		case "CONTENT_TYPES":
			cfg.ContentTypes = splitCSV(val)
		case "MAX_ITEMS_PER_SYNC":
			cfg.MaxItemsPerSync = parsePositiveInt(key, val, cfg.MaxItemsPerSync)
		case "AUTO_SYNC_ENABLED":
			cfg.AutoSyncEnabled = parseBool(val)
		case "SYNC_INTERVAL_MINUTES":
			cfg.SyncIntervalMinutes = parsePositiveInt(key, val, cfg.SyncIntervalMinutes)
		case "NETWORK_WAIT_SECONDS":
			cfg.NetworkWaitSeconds = parseNonNegativeInt(key, val, cfg.NetworkWaitSeconds)
		case "AUTO_CONNECT_WIFI":
			cfg.AutoConnectWiFi = parseBool(val)
		case "TRIGGER_KOBO_RESCAN":
			cfg.TriggerKoboRescan = parseBool(val)
		}
	}

	if cfg.APIBaseURL == "" {
		return cfg, errors.New("API_BASE_URL cannot be empty")
	}
	if cfg.OutputDir == "" {
		return cfg, errors.New("OUTPUT_DIR cannot be empty")
	}
	if cfg.StatePath == "" {
		return cfg, errors.New("STATE_PATH cannot be empty")
	}
	if cfg.MaxItemsPerSync < 1 {
		cfg.MaxItemsPerSync = 1
	}
	if cfg.SyncIntervalMinutes < 5 {
		cfg.SyncIntervalMinutes = 5
	}
	return cfg, nil
}

func splitCSV(val string) []string {
	var out []string
	for _, part := range strings.Split(val, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseBool(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parsePositiveInt(key, val string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || n < 1 {
		log.Printf("invalid %s=%q; using %d", key, val, fallback)
		return fallback
	}
	return n
}

func parseNonNegativeInt(key, val string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil || n < 0 {
		log.Printf("invalid %s=%q; using %d", key, val, fallback)
		return fallback
	}
	return n
}

func readState(path string) State {
	state := State{Items: map[string]ItemState{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return state
	}
	if err := json.Unmarshal(data, &state); err != nil {
		log.Printf("could not parse state %s: %v", path, err)
		return State{Items: map[string]ItemState{}}
	}
	if state.Items == nil {
		state.Items = map[string]ItemState{}
	}
	return state
}

func writeState(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func validateToken(cfg Config) error {
	if strings.TrimSpace(cfg.MatterToken) == "" {
		return errors.New("MATTER_TOKEN is missing in config.env")
	}
	if !strings.HasPrefix(cfg.MatterToken, "mat_") {
		log.Printf("warning: MATTER_TOKEN does not start with mat_")
	}
	return nil
}

func newMatterClient(cfg Config) MatterClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if certPool, err := loadRootCAs(cfg.CACertFile); err != nil {
		log.Printf("warning: could not load CA bundle: %v", err)
	} else if certPool != nil {
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    certPool,
		}
	}

	return MatterClient{
		baseURL: strings.TrimRight(cfg.APIBaseURL, "/"),
		token:   cfg.MatterToken,
		http: &http.Client{
			Timeout:   45 * time.Second,
			Transport: transport,
		},
	}
}

func loadRootCAs(path string) (*x509.CertPool, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if ok := pool.AppendCertsFromPEM(data); !ok {
		return nil, fmt.Errorf("no PEM certificates found in %s", path)
	}
	log.Printf("loaded CA bundle: %s", path)
	return pool, nil
}

func (c MatterClient) getJSON(ctx context.Context, apiPath string, params url.Values, target any) error {
	endpoint := c.baseURL + apiPath
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "matter-kobo-sync/"+version)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(backoff(attempt))
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			wait := retryAfter(resp.Header.Get("Retry-After"), 10*time.Second)
			log.Printf("rate limited; waiting %s", wait)
			time.Sleep(wait)
			lastErr = fmt.Errorf("rate limited")
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("matter returned %d: %s", resp.StatusCode, trimForLog(body))
			time.Sleep(backoff(attempt))
			continue
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("matter returned %d: %s", resp.StatusCode, trimForLog(body))
		}
		if err := json.Unmarshal(body, target); err != nil {
			return fmt.Errorf("decode Matter response: %w", err)
		}
		return nil
	}
	return lastErr
}

func trimForLog(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 500 {
		return text[:500] + "..."
	}
	return text
}

func backoff(attempt int) time.Duration {
	return time.Duration(1<<attempt) * time.Second
}

func retryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}
	if n, err := strconv.Atoi(header); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return fallback
}

func (c MatterClient) listItems(ctx context.Context, cfg Config, state State) ([]MatterItem, error) {
	params := url.Values{}
	params.Set("status", cfg.SyncStatus)
	params.Set("limit", "100")
	params.Set("order", "library_position")
	if len(cfg.ContentTypes) > 0 {
		params.Set("content_type", strings.Join(cfg.ContentTypes, ","))
	}

	var out []MatterItem
	for {
		var page listResponse
		if err := c.getJSON(ctx, "/items", params, &page); err != nil {
			return nil, err
		}
		out = append(out, page.Results...)
		if !page.HasMore || page.NextCursor == "" {
			break
		}
		params.Set("cursor", page.NextCursor)
	}
	return out, nil
}

func (c MatterClient) getItemMarkdown(ctx context.Context, id string) (MatterItem, error) {
	params := url.Values{}
	params.Set("include", "markdown")
	var item MatterItem
	err := c.getJSON(ctx, "/items/"+url.PathEscape(id), params, &item)
	return item, err
}

func runOnce(ctx context.Context, cfg Config) error {
	if err := validateToken(cfg); err != nil {
		return err
	}
	if cfg.AutoConnectWiFi {
		if err := triggerWiFiConnect(); err != nil {
			log.Printf("wifi autoconnect not triggered: %v", err)
		}
	}
	if err := waitForNetwork(ctx, cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return err
	}

	state := readState(cfg.StatePath)
	startedAt := time.Now().UTC().Format(time.RFC3339)
	state.LastSyncStartedAt = startedAt
	state.LastError = ""
	if err := writeState(cfg.StatePath, state); err != nil {
		return err
	}
	client := newMatterClient(cfg)

	log.Printf("sync started")
	items, err := client.listItems(ctx, cfg, state)
	if err != nil {
		state.LastError = err.Error()
		_ = writeState(cfg.StatePath, state)
		return err
	}
	log.Printf("matter returned %d changed item(s)", len(items))

	written := 0
	markdownFetches := 0
	currentItems := make(map[string]MatterItem, len(items))
	for _, item := range items {
		currentItems[item.ID] = item
	}
	for i, item := range items {
		prev := state.Items[item.ID]
		if prev.UpdatedAt == item.UpdatedAt && prev.EPUBMetadataVersion == epubMetadataVersion && prev.FilePath != "" && fileExists(prev.FilePath) {
			log.Printf("unchanged: %s", item.Title)
			continue
		}
		if cfg.MaxItemsPerSync > 0 && markdownFetches >= cfg.MaxItemsPerSync {
			log.Printf("markdown fetch limit reached; remaining changed items will sync later")
			break
		}
		markdownFetches++
		full, err := client.getItemMarkdown(ctx, item.ID)
		if err != nil {
			state.LastError = err.Error()
			_ = writeState(cfg.StatePath, state)
			return err
		}
		if strings.TrimSpace(full.Markdown) == "" {
			log.Printf("skipping %s: markdown empty or item still processing", displayTitle(full))
			continue
		}
		path, err := writeEPUB(cfg.OutputDir, full)
		if err != nil {
			state.LastError = err.Error()
			_ = writeState(cfg.StatePath, state)
			return err
		}
		state.Items[full.ID] = ItemState{
			ID:                  full.ID,
			Title:               displayTitle(full),
			UpdatedAt:           full.UpdatedAt,
			FilePath:            path,
			EPUBMetadataVersion: epubMetadataVersion,
		}
		if err := writeState(cfg.StatePath, state); err != nil {
			return err
		}
		written++
		log.Printf("wrote %s", path)

		if i < len(items)-1 {
			time.Sleep(3 * time.Second)
		}
	}
	removed, err := removeMissingItems(cfg.OutputDir, &state, currentItems)
	if err != nil {
		state.LastError = err.Error()
		_ = writeState(cfg.StatePath, state)
		return err
	}

	state.LastSyncStartedAt = startedAt
	state.LastSyncFinishedAt = time.Now().UTC().Format(time.RFC3339)
	state.LastError = ""
	if err := writeState(cfg.StatePath, state); err != nil {
		return err
	}
	if (written > 0 || removed > 0) && cfg.TriggerKoboRescan {
		if err := triggerKoboRescan(); err != nil {
			log.Printf("library rescan not triggered: %v", err)
		}
	}
	log.Printf("sync finished; wrote %d EPUB(s), removed %d stale EPUB(s)", written, removed)
	return nil
}

func removeMissingItems(outputDir string, state *State, currentItems map[string]MatterItem) (int, error) {
	deletedFiles := 0
	for id, item := range state.Items {
		if _, ok := currentItems[id]; ok {
			continue
		}
		if item.FilePath != "" && isPathUnderDir(item.FilePath, outputDir) {
			if err := os.Remove(item.FilePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return deletedFiles, fmt.Errorf("remove stale EPUB %s: %w", item.FilePath, err)
			}
			deletedFiles++
			log.Printf("removed stale EPUB: %s", item.FilePath)
		} else if item.FilePath != "" {
			log.Printf("forgetting stale item with unsafe path outside output dir: %s", item.FilePath)
		}
		delete(state.Items, id)
	}
	return deletedFiles, nil
}

func isPathUnderDir(path, dir string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func runDaemon(ctx context.Context, configPath string, cfg Config) error {
	if !cfg.AutoSyncEnabled {
		log.Printf("auto sync disabled")
		return nil
	}
	if cfg.SyncIntervalMinutes < 5 {
		cfg.SyncIntervalMinutes = 5
	}
	log.Printf("daemon started; interval=%d minutes", cfg.SyncIntervalMinutes)
	for {
		latest, err := loadConfig(configPath)
		if err != nil {
			log.Printf("could not reload config: %v", err)
		} else {
			cfg = latest
		}
		if !cfg.AutoSyncEnabled {
			log.Printf("auto sync disabled")
			return nil
		}

		delay := time.Duration(cfg.SyncIntervalMinutes) * time.Minute
		if strings.TrimSpace(cfg.MatterToken) == "" {
			log.Printf("MATTER_TOKEN is missing in config.env")
			delay = minDuration(delay, time.Minute)
		} else if err := runOnce(ctx, cfg); err != nil {
			log.Printf("sync failed: %v", err)
			delay = minDuration(delay, 5*time.Minute)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func waitForNetwork(ctx context.Context, cfg Config) error {
	if cfg.NetworkWaitSeconds == 0 {
		return nil
	}
	parsed, err := url.Parse(cfg.APIBaseURL)
	if err != nil {
		return err
	}
	host := parsed.Host
	if host == "" {
		return fmt.Errorf("API_BASE_URL has no host: %s", cfg.APIBaseURL)
	}
	if !strings.Contains(host, ":") {
		port := "443"
		if parsed.Scheme == "http" {
			port = "80"
		}
		host = net.JoinHostPort(host, port)
	}

	deadline := time.Now().Add(time.Duration(cfg.NetworkWaitSeconds) * time.Second)
	var lastErr error
	for {
		dialer := net.Dialer{Timeout: 8 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", host)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("network unavailable after %d seconds: %w", cfg.NetworkWaitSeconds, lastErr)
		}
		log.Printf("waiting for network: %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func printStatus(cfg Config) error {
	state := readState(cfg.StatePath)
	if state.LastSyncFinishedAt == "" {
		if state.LastSyncStartedAt == "" {
			fmt.Println("Matter sync has not completed yet.")
		} else {
			fmt.Printf("Matter sync started at %s and has not completed yet.\n", state.LastSyncStartedAt)
		}
	} else {
		fmt.Printf("Last sync: %s\n", state.LastSyncFinishedAt)
	}
	if state.LastError != "" {
		fmt.Printf("Last error: %s\n", state.LastError)
	}
	fmt.Printf("Synced items: %d\n", len(state.Items))
	fmt.Printf("Output: %s\n", cfg.OutputDir)
	if cfg.MatterToken == "" {
		fmt.Println("Token: missing")
	} else {
		fmt.Println("Token: configured")
	}
	return nil
}

func testMatter(ctx context.Context, cfg Config) error {
	if err := validateToken(cfg); err != nil {
		return err
	}
	client := newMatterClient(cfg)
	var account accountResponse
	if err := client.getJSON(ctx, "/me", nil, &account); err != nil {
		return err
	}
	fmt.Printf("Matter account: %s <%s>\n", account.Name, account.Email)
	fmt.Printf("Matter Pro: %t\n", account.IsPro)
	return nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func displayTitle(item MatterItem) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = strings.TrimSpace(item.URL)
	}
	if title == "" {
		title = item.ID
	}
	return title
}

func writeEPUB(outputDir string, item MatterItem) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	fileName := safeFilename(displayTitle(item))
	if fileName == "" {
		fileName = item.ID
	}
	fileName = fmt.Sprintf("%s [%s].epub", fileName, item.ID)
	path := filepath.Join(outputDir, fileName)
	tmp := path + ".tmp"

	epub, err := buildEPUB(item)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(tmp, epub, 0644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)

func safeFilename(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = unsafeFileChars.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 100 {
		s = strings.TrimSpace(s[:100])
	}
	return s
}

func buildEPUB(item MatterItem) ([]byte, error) {
	title := displayTitle(item)
	identifier := "matter:" + item.ID
	modified := epubTimestamp(item.UpdatedAt)
	body := markdownToXHTML(item.Markdown)

	article := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="styles.css"/>
</head>
<body>
  <section epub:type="bodymatter">
    <h1>%s</h1>
    <p class="meta">%s</p>
    %s
  </section>
</body>
</html>
`, escapeXML(title), escapeXML(title), escapeXML(metaLine(item)), body)

	nav := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en">
<head><title>%s</title></head>
<body>
  <nav epub:type="toc" id="toc">
    <h1>Contents</h1>
    <ol><li><a href="article.xhtml">%s</a></li></ol>
  </nav>
</body>
</html>
`, escapeXML(title), escapeXML(title))

	ncx := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN"
  "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head>
    <meta name="dtb:uid" content="%s"/>
    <meta name="dtb:depth" content="1"/>
    <meta name="dtb:totalPageCount" content="0"/>
    <meta name="dtb:maxPageNumber" content="0"/>
  </head>
  <docTitle><text>%s</text></docTitle>
  <navMap>
    <navPoint id="navpoint-1" playOrder="1">
      <navLabel><text>%s</text></navLabel>
      <content src="article.xhtml"/>
    </navPoint>
  </navMap>
</ncx>
`, escapeXML(identifier), escapeXML(title), escapeXML(title))

	opf := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="uid">%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>Matter</dc:creator>
    <dc:publisher>Matter</dc:publisher>
    <dc:subject>Matter</dc:subject>
    <dc:source>Matter</dc:source>
    <dc:language>en</dc:language>
    <meta name="calibre:series" content="Matter"/>
    <meta property="belongs-to-collection" id="matter-collection">Matter</meta>
    <meta refines="#matter-collection" property="collection-type">series</meta>
    <meta property="dcterms:modified">%s</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="article" href="article.xhtml" media-type="application/xhtml+xml"/>
    <item id="styles" href="styles.css" media-type="text/css"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="article"/>
  </spine>
</package>
`, escapeXML(identifier), escapeXML(title), modified)

	container := `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`
	css := `body { font-family: serif; line-height: 1.35; }
h1, h2, h3, h4, h5, h6 { line-height: 1.2; }
.meta { color: #555; font-size: 0.85em; }
blockquote { border-left: 0.2em solid #999; margin-left: 0; padding-left: 1em; }
pre { white-space: pre-wrap; font-family: monospace; }
img { max-width: 100%; }
`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})

	if err := addZipFile(zw, "mimetype", []byte("application/epub+zip"), zip.Store); err != nil {
		return nil, err
	}
	files := map[string][]byte{
		"META-INF/container.xml": []byte(container),
		"OEBPS/content.opf":      []byte(opf),
		"OEBPS/nav.xhtml":        []byte(nav),
		"OEBPS/toc.ncx":          []byte(ncx),
		"OEBPS/article.xhtml":    []byte(article),
		"OEBPS/styles.css":       []byte(css),
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		method := zip.Deflate
		if err := addZipFile(zw, name, files[name], method); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func addZipFile(zw *zip.Writer, name string, data []byte, method uint16) error {
	header := &zip.FileHeader{Name: name, Method: method}
	header.SetMode(0644)
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func epubTimestamp(value string) string {
	if value != "" {
		if t, err := time.Parse(time.RFC3339, value); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05Z")
		}
	}
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func metaLine(item MatterItem) string {
	parts := []string{"Synced from Matter"}
	if item.Author.Name != "" {
		parts = append(parts, item.Author.Name)
	} else if item.SiteName != "" {
		parts = append(parts, item.SiteName)
	}
	if item.URL != "" {
		parts = append(parts, item.URL)
	}
	return strings.Join(parts, " | ")
}

func escapeXML(s string) string {
	return html.EscapeString(s)
}

func markdownToXHTML(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	var b strings.Builder
	var paragraph []string
	inUL := false
	inOL := false
	inPre := false

	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(inlineMarkdown(strings.Join(paragraph, " ")))
		b.WriteString("</p>\n")
		paragraph = nil
	}
	closeLists := func() {
		if inUL {
			b.WriteString("</ul>\n")
			inUL = false
		}
		if inOL {
			b.WriteString("</ol>\n")
			inOL = false
		}
	}

	ordered := regexp.MustCompile(`^\d+\.\s+`)
	for _, raw := range lines {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			flushParagraph()
			closeLists()
			if inPre {
				b.WriteString("</code></pre>\n")
				inPre = false
			} else {
				b.WriteString("<pre><code>")
				inPre = true
			}
			continue
		}
		if inPre {
			b.WriteString(escapeXML(line))
			b.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			flushParagraph()
			closeLists()
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 0 && level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				flushParagraph()
				closeLists()
				text := strings.TrimSpace(trimmed[level:])
				fmt.Fprintf(&b, "<h%d>%s</h%d>\n", level, inlineMarkdown(text), level)
				continue
			}
		}
		if strings.HasPrefix(trimmed, ">") {
			flushParagraph()
			closeLists()
			text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			b.WriteString("<blockquote><p>")
			b.WriteString(inlineMarkdown(text))
			b.WriteString("</p></blockquote>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushParagraph()
			if inOL {
				b.WriteString("</ol>\n")
				inOL = false
			}
			if !inUL {
				b.WriteString("<ul>\n")
				inUL = true
			}
			b.WriteString("<li>")
			b.WriteString(inlineMarkdown(strings.TrimSpace(trimmed[2:])))
			b.WriteString("</li>\n")
			continue
		}
		if loc := ordered.FindStringIndex(trimmed); loc != nil && loc[0] == 0 {
			flushParagraph()
			if inUL {
				b.WriteString("</ul>\n")
				inUL = false
			}
			if !inOL {
				b.WriteString("<ol>\n")
				inOL = true
			}
			b.WriteString("<li>")
			b.WriteString(inlineMarkdown(strings.TrimSpace(trimmed[loc[1]:])))
			b.WriteString("</li>\n")
			continue
		}

		closeLists()
		paragraph = append(paragraph, trimmed)
	}
	flushParagraph()
	closeLists()
	if inPre {
		b.WriteString("</code></pre>\n")
	}
	return b.String()
}

func inlineMarkdown(s string) string {
	var out strings.Builder
	for len(s) > 0 {
		open := strings.Index(s, "[")
		if open < 0 {
			out.WriteString(escapeXML(s))
			break
		}
		closeText := strings.Index(s[open:], "](")
		if closeText < 0 {
			out.WriteString(escapeXML(s))
			break
		}
		closeText += open
		closeURL := strings.Index(s[closeText+2:], ")")
		if closeURL < 0 {
			out.WriteString(escapeXML(s))
			break
		}
		closeURL += closeText + 2
		out.WriteString(escapeXML(s[:open]))
		label := s[open+1 : closeText]
		href := s[closeText+2 : closeURL]
		out.WriteString(`<a href="`)
		out.WriteString(escapeXML(href))
		out.WriteString(`">`)
		out.WriteString(escapeXML(label))
		out.WriteString(`</a>`)
		s = s[closeURL+1:]
	}
	return out.String()
}

func triggerKoboRescan() error {
	path, err := findQndb()
	if err != nil {
		return err
	}
	output, err := exec.Command(path, "-t", "60000", "-s", "pfmDoneProcessing", "-m", "pfmRescanBooksFull").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", path, err, trimForLog(output))
	}
	log.Printf("triggered Kobo library rescan via %s", path)
	return nil
}

func triggerWiFiConnect() error {
	path, err := findQndb()
	if err != nil {
		return err
	}
	output, err := exec.Command(path, "-m", "wfmConnectWirelessSilently").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w: %s", path, err, trimForLog(output))
	}
	log.Printf("triggered Wi-Fi autoconnect via %s", path)
	return nil
}

func findQndb() (string, error) {
	candidates := []string{
		"/usr/local/bin/qndb",
		"/usr/bin/qndb",
		"/mnt/onboard/.adds/nickeldbus/qndb",
		"/mnt/onboard/.adds/nickeldbus/bin/qndb",
	}
	if path, err := exec.LookPath("qndb"); err == nil {
		candidates = append([]string{path}, candidates...)
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		st, err := os.Stat(candidate)
		if err == nil && !st.IsDir() && st.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	return "", errors.New("qndb not found; NickelDBus may not be installed or loaded")
}
