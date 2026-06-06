package main

import (
	"context"
	"fmt"
	"time"

	"github.com/open-code-review/open-code-review/internal/explain"
	"github.com/open-code-review/open-code-review/internal/llm"
	"github.com/open-code-review/open-code-review/internal/stdout"
	"github.com/open-code-review/open-code-review/internal/telemetry"
)

func runExplain(args []string) error {
	opts, err := parseExplainFlags(args)
	if err != nil {
		return fmt.Errorf("parse explain flags: %w", err)
	}
	if opts.showHelp {
		printExplainUsage()
		return nil
	}
	if err := requireGitRepo(opts.repoDir); err != nil {
		return err
	}
	repoDir, err := resolveRepoDir(opts.repoDir)
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}

	cfgPath, err := defaultConfigPath()
	if err != nil {
		return err
	}
	ep, err := llm.ResolveEndpoint(cfgPath)
	if err != nil && !opts.preview {
		return fmt.Errorf("resolve LLM endpoint: %w", err)
	}

	var client llm.LLMClient
	model := ""
	if err == nil {
		client = llm.NewLLMClient(ep)
		model = ep.Model
	}

	var unsilence func()
	if opts.outputFormat == "json" || opts.audience == "agent" {
		unsilence = stdout.Quiet()
		defer func() {
			if unsilence != nil {
				unsilence()
			}
		}()
	}

	ctx, span := telemetry.StartSpan(context.Background(), "explain.run")
	defer span.End()

	timeout := time.Duration(opts.perRunTimeout) * time.Minute
	ag := explain.New(explain.Args{
		RepoDir:       repoDir,
		Targets:       opts.targets,
		AllFiles:      opts.allFiles,
		GraphPath:     opts.graphPath,
		OutDir:        opts.outDir,
		Format:        opts.outputFormat,
		Audience:      opts.audience,
		Background:    opts.background,
		MaxFiles:      opts.maxFiles,
		MaxTools:      opts.maxTools,
		Timeout:       timeout,
		Preview:       opts.preview,
		Ref:           opts.ref,
		LLMClient:     client,
		Model:         model,
		AllowGraphRun: opts.allFiles,
	})

	result, err := ag.Run(ctx)
	if err != nil {
		telemetry.SetAttr(span, "error", err.Error())
		return fmt.Errorf("explain failed: %w", err)
	}

	if opts.audience == "agent" && opts.outputFormat != "json" && unsilence != nil {
		unsilence()
		unsilence = nil
	}

	if opts.outputFormat == "json" {
		return explain.PrintJSON(result)
	}
	explain.PrintText(result)
	return nil
}
