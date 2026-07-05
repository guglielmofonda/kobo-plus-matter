package main

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEPUB(t *testing.T) {
	item := MatterItem{
		ID:        "itm_test",
		Title:     "Example Article",
		URL:       "https://example.com/article",
		SiteName:  "example.com",
		UpdatedAt: "2026-06-14T12:00:00Z",
		Markdown:  "# Heading\n\nA paragraph with [a link](https://example.com).\n\n- one\n- two\n",
	}
	data, err := buildEPUB(item)
	if err != nil {
		t.Fatal(err)
	}

	tmp := filepath.Join(t.TempDir(), "article.epub")
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	names := map[string]bool{}
	entries := map[string]string{}
	for idx, f := range zr.File {
		if idx == 0 && f.Name != "mimetype" {
			t.Fatalf("mimetype must be first EPUB entry, got %s", f.Name)
		}
		if f.Name == "mimetype" && f.Method != zip.Store {
			t.Fatal("mimetype must be stored without compression")
		}
		names[f.Name] = true
		switch f.Name {
		case "META-INF/container.xml", "OEBPS/content.opf", "OEBPS/nav.xhtml", "OEBPS/toc.ncx", "OEBPS/article.xhtml":
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf, err := io.ReadAll(rc)
			if err != nil {
				t.Fatal(err)
			}
			_ = rc.Close()
			entries[f.Name] = string(buf)
		}
	}
	for _, required := range []string{
		"mimetype",
		"META-INF/container.xml",
		"OEBPS/content.opf",
		"OEBPS/nav.xhtml",
		"OEBPS/toc.ncx",
		"OEBPS/article.xhtml",
		"OEBPS/styles.css",
	} {
		if !names[required] {
			t.Fatalf("missing EPUB entry %s", required)
		}
	}
	for name, content := range entries {
		if err := parseXML(content); err != nil {
			t.Fatalf("%s is not well-formed XML: %v\n%s", name, err, content)
		}
	}
	article := entries["OEBPS/article.xhtml"]
	if !strings.Contains(article, "<h1>Example Article</h1>") {
		t.Fatalf("article title missing from xhtml: %s", article)
	}
	if !strings.Contains(article, `<a href="https://example.com">a link</a>`) {
		t.Fatalf("markdown link not converted: %s", article)
	}
	if !strings.Contains(article, `xmlns:epub="http://www.idpf.org/2007/ops"`) {
		t.Fatalf("article XHTML missing epub namespace: %s", article)
	}
	opf := entries["OEBPS/content.opf"]
	if !strings.Contains(opf, `id="ncx"`) || !strings.Contains(opf, `<spine toc="ncx">`) {
		t.Fatalf("OPF missing NCX compatibility wiring: %s", opf)
	}
	for _, marker := range []string{
		"<dc:creator>Matter</dc:creator>",
		"<dc:publisher>Matter</dc:publisher>",
		"<dc:subject>Matter</dc:subject>",
		`<meta name="calibre:series" content="Matter"/>`,
		`<meta property="belongs-to-collection" id="matter-collection">Matter</meta>`,
	} {
		if !strings.Contains(opf, marker) {
			t.Fatalf("OPF missing Matter metadata %q: %s", marker, opf)
		}
	}
}

func parseXML(content string) error {
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		if _, err := decoder.Token(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.env")
	err := os.WriteFile(configPath, []byte(`
MATTER_TOKEN=mat_test
OUTPUT_DIR=/tmp/matter
CA_CERT_FILE=/tmp/custom-ca.pem
CONTENT_TYPES=article,newsletter
MAX_ITEMS_PER_SYNC=12
AUTO_SYNC_ENABLED=1
NETWORK_WAIT_SECONDS=0
AUTO_CONNECT_WIFI=1
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MatterToken != "mat_test" {
		t.Fatalf("token mismatch: %q", cfg.MatterToken)
	}
	if cfg.MaxItemsPerSync != 12 {
		t.Fatalf("max items mismatch: %d", cfg.MaxItemsPerSync)
	}
	if cfg.CACertFile != "/tmp/custom-ca.pem" {
		t.Fatalf("CA cert path mismatch: %q", cfg.CACertFile)
	}
	if strings.Join(cfg.ContentTypes, ",") != "article,newsletter" {
		t.Fatalf("content types mismatch: %#v", cfg.ContentTypes)
	}
	if !cfg.AutoSyncEnabled {
		t.Fatal("auto sync should be enabled")
	}
	if cfg.NetworkWaitSeconds != 0 {
		t.Fatalf("network wait mismatch: %d", cfg.NetworkWaitSeconds)
	}
	if !cfg.AutoConnectWiFi {
		t.Fatal("auto connect wifi should be enabled")
	}
}

func TestLoadRootCAsRejectsInvalidBundle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cacert.pem")
	if err := os.WriteFile(path, []byte("not a cert"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRootCAs(path); err == nil {
		t.Fatal("expected invalid CA bundle error")
	}
}

func TestTriggerWiFiConnectUsesQndb(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "args.log")
	qndbPath := filepath.Join(dir, "qndb")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > " + shellQuote(logPath) + "\n"
	if err := os.WriteFile(qndbPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)

	if err := triggerWiFiConnect(); err != nil {
		t.Fatal(err)
	}
	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(args)) != "-m wfmConnectWirelessSilently" {
		t.Fatalf("unexpected qndb args: %q", string(args))
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func TestRemoveMissingItemsDeletesOnlyInsideOutputDir(t *testing.T) {
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "Matter")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Fatal(err)
	}

	keepPath := filepath.Join(outputDir, "Keep [itm_keep].epub")
	removePath := filepath.Join(outputDir, "Remove [itm_remove].epub")
	outsidePath := filepath.Join(dir, "outside.epub")
	for _, path := range []string{keepPath, removePath, outsidePath} {
		if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	state := State{Items: map[string]ItemState{
		"itm_keep": {
			ID:       "itm_keep",
			Title:    "Keep",
			FilePath: keepPath,
		},
		"itm_remove": {
			ID:       "itm_remove",
			Title:    "Remove",
			FilePath: removePath,
		},
		"itm_outside": {
			ID:       "itm_outside",
			Title:    "Outside",
			FilePath: outsidePath,
		},
	}}

	deleted, err := removeMissingItems(outputDir, &state, map[string]MatterItem{
		"itm_keep": {ID: "itm_keep"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted count mismatch: %d", deleted)
	}
	if !fileExists(keepPath) {
		t.Fatal("kept item file was removed")
	}
	if fileExists(removePath) {
		t.Fatal("stale item file was not removed")
	}
	if !fileExists(outsidePath) {
		t.Fatal("outside file should not be removed")
	}
	if _, ok := state.Items["itm_keep"]; !ok {
		t.Fatal("current state item was removed")
	}
	if _, ok := state.Items["itm_remove"]; ok {
		t.Fatal("stale state item was not removed")
	}
	if _, ok := state.Items["itm_outside"]; ok {
		t.Fatal("unsafe outside state item was not forgotten")
	}
}

func TestIsPathUnderDir(t *testing.T) {
	dir := t.TempDir()
	inside := filepath.Join(dir, "Matter", "article.epub")
	outside := filepath.Join(dir, "Other", "article.epub")
	if !isPathUnderDir(inside, filepath.Join(dir, "Matter")) {
		t.Fatal("inside path was not accepted")
	}
	if isPathUnderDir(outside, filepath.Join(dir, "Matter")) {
		t.Fatal("outside path was accepted")
	}
}
