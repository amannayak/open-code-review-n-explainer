package main

import "testing"

func TestParseExplainFlags_Defaults(t *testing.T) {
	opts, err := parseExplainFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if opts.outputFormat != "text" {
		t.Fatalf("format = %q, want text", opts.outputFormat)
	}
	if opts.audience != "human" {
		t.Fatalf("audience = %q, want human", opts.audience)
	}
	if opts.maxFiles != 50 {
		t.Fatalf("maxFiles = %d, want 50", opts.maxFiles)
	}
}

func TestParseExplainFlags_TargetsAndGraph(t *testing.T) {
	opts, err := parseExplainFlags([]string{
		"--all-files",
		"--graph", "graphify-out/graph.json",
		"--out", "explain-out",
		"--format", "json",
		"--audience", "agent",
		"--max-files", "7",
		"internal/agent",
		"cmd/opencodereview",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.allFiles {
		t.Fatal("allFiles = false, want true")
	}
	if opts.graphPath != "graphify-out/graph.json" {
		t.Fatalf("graphPath = %q", opts.graphPath)
	}
	if opts.outDir != "explain-out" {
		t.Fatalf("outDir = %q", opts.outDir)
	}
	if opts.outputFormat != "json" || opts.audience != "agent" {
		t.Fatalf("format/audience = %q/%q", opts.outputFormat, opts.audience)
	}
	if opts.maxFiles != 7 {
		t.Fatalf("maxFiles = %d", opts.maxFiles)
	}
	if len(opts.targets) != 2 {
		t.Fatalf("targets = %v", opts.targets)
	}
}

func TestParseExplainFlags_Invalid(t *testing.T) {
	if _, err := parseExplainFlags([]string{"--format", "xml"}); err == nil {
		t.Fatal("expected invalid format error")
	}
	if _, err := parseExplainFlags([]string{"--audience", "bot"}); err == nil {
		t.Fatal("expected invalid audience error")
	}
	if _, err := parseExplainFlags([]string{"--max-files", "0"}); err == nil {
		t.Fatal("expected max-files error")
	}
}

func TestRunSetupHelpAndUnknown(t *testing.T) {
	if err := runSetup([]string{}); err != nil {
		t.Fatalf("help setup returned error: %v", err)
	}
	if err := runSetup([]string{"unknown"}); err == nil {
		t.Fatal("expected unknown setup target error")
	}
}

func TestSetConfigValueAllowLocalNoToken(t *testing.T) {
	cfg := &Config{}
	if err := setConfigValue(cfg, "llm.allow_local_no_token", "true"); err != nil {
		t.Fatal(err)
	}
	if !cfg.Llm.AllowLocalNoToken {
		t.Fatal("AllowLocalNoToken = false, want true")
	}
}
