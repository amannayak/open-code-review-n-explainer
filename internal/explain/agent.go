package explain

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/open-code-review/open-code-review/internal/llm"
	"github.com/open-code-review/open-code-review/internal/session"
	"github.com/open-code-review/open-code-review/internal/stdout"
	"github.com/open-code-review/open-code-review/internal/tool"
)

const (
	defaultMaxExplainFiles = 50
	defaultMaxToolRounds   = 12
	maxInlineFileBytes     = 24000
)

// Args holds dependencies and settings for tutor mode.
type Args struct {
	RepoDir       string
	Targets       []string
	AllFiles      bool
	GraphPath     string
	OutDir        string
	Format        string
	Audience      string
	Background    string
	MaxFiles      int
	MaxTools      int
	Timeout       time.Duration
	Preview       bool
	Ref           string
	LLMClient     llm.LLMClient
	Model         string
	Session       *session.SessionHistory
	AllowGraphRun bool
}

// Agent orchestrates read-only tutor explanations.
type Agent struct {
	args              Args
	graph             *Graph
	session           *session.SessionHistory
	totalInputTokens  int64
	totalOutputTokens int64
	totalTokens       int64
	warnings          []string
}

// New creates an explain agent.
func New(args Args) *Agent {
	if args.MaxFiles <= 0 {
		args.MaxFiles = defaultMaxExplainFiles
	}
	if args.MaxTools <= 0 {
		args.MaxTools = defaultMaxToolRounds
	}
	if args.Session == nil {
		args.Session = session.New(args.RepoDir, detectGitBranch(args.RepoDir), args.Model, session.SessionOptions{
			ReviewMode: "explain",
			DiffTo:     args.Ref,
		})
	}
	return &Agent{args: args, session: args.Session}
}

// Run performs the tutor-mode explanation.
func (a *Agent) Run(ctx context.Context) (*Result, error) {
	start := time.Now()
	if a.args.AllFiles || a.args.GraphPath != "" {
		if err := a.ensureGraph(); err != nil {
			return nil, err
		}
	}

	inventory, err := a.selectInventory()
	if err != nil {
		return nil, err
	}
	if len(inventory) == 0 {
		return nil, fmt.Errorf("no explainable files found")
	}
	if a.args.Preview {
		return a.previewResult(inventory, start), nil
	}

	ctx = a.withTimeout(ctx)
	var files []FileExplanation
	for _, item := range inventory {
		explained, err := a.explainFile(ctx, item.Path)
		if err != nil {
			a.warnings = append(a.warnings, fmt.Sprintf("%s: %v", item.Path, err))
			continue
		}
		files = append(files, explained)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("all file explanations failed")
	}

	architecture, summary, flows, err := a.explainArchitecture(ctx, files)
	if err != nil {
		a.warnings = append(a.warnings, fmt.Sprintf("architecture synthesis: %v", err))
		architecture = fallbackArchitecture(files, a.graph)
		summary = "Generated file-by-file explanations; architecture synthesis used deterministic fallback."
	}

	result := &Result{
		Status:         "success",
		Summary:        summary,
		Architecture:   architecture,
		Files:          files,
		Relationships:  a.relationshipsForOutput(),
		Flows:          flows,
		Warnings:       a.warnings,
		GraphPath:      a.args.GraphPath,
		FilesExplained: int64(len(files)),
		InputTokens:    atomic.LoadInt64(&a.totalInputTokens),
		OutputTokens:   atomic.LoadInt64(&a.totalOutputTokens),
		TotalTokens:    atomic.LoadInt64(&a.totalTokens),
		Elapsed:        time.Since(start).Round(time.Second).String(),
	}
	if a.args.OutDir != "" {
		if err := WriteArtifacts(a.args.RepoDir, a.args.OutDir, result); err != nil {
			return nil, err
		}
	}
	a.session.Finalize()
	return result, nil
}

func (a *Agent) withTimeout(ctx context.Context) context.Context {
	if a.args.Timeout <= 0 {
		return ctx
	}
	tctx, _ := context.WithTimeout(ctx, a.args.Timeout)
	return tctx
}

func (a *Agent) ensureGraph() error {
	graphPath := a.args.GraphPath
	if graphPath == "" {
		graphPath = DefaultGraphPath(a.args.RepoDir)
	}
	if !filepath.IsAbs(graphPath) {
		graphPath = filepath.Join(a.args.RepoDir, graphPath)
	}
	if _, err := os.Stat(graphPath); err != nil {
		if !a.args.AllFiles {
			return fmt.Errorf("graph file not found: %s", graphPath)
		}
		if !a.args.AllowGraphRun {
			return fmt.Errorf("graph file not found: %s", graphPath)
		}
		if err := runGraphify(a.args.RepoDir); err != nil {
			return err
		}
	}
	graph, err := LoadGraph(graphPath)
	if err != nil {
		return err
	}
	a.graph = graph
	a.args.GraphPath = graphPath
	return nil
}

func runGraphify(repoDir string) error {
	bin, err := findGraphifyBinary()
	if err != nil {
		return fmt.Errorf("Graphify is required for --all-files explanation. Install it with: uv tool install graphifyy")
	}
	cmd := exec.Command(bin, ".", "--no-viz")
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("run graphify: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func findGraphifyBinary() (string, error) {
	for _, name := range []string{"graphify", "graphifyy"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("graphify executable not found")
}

func (a *Agent) selectInventory() ([]InventoryFile, error) {
	targets := a.args.Targets
	if a.args.AllFiles && a.graph != nil {
		graphFiles := a.graph.FilePaths()
		if len(graphFiles) > 0 {
			targets = graphFiles
		}
	}
	maxFiles := a.args.MaxFiles
	if a.args.AllFiles && len(targets) > 0 && len(targets) < maxFiles {
		maxFiles = len(targets)
	}
	return Inventory(a.args.RepoDir, a.args.Ref, targets, maxFiles)
}

func (a *Agent) previewResult(files []InventoryFile, start time.Time) *Result {
	out := make([]FileExplanation, 0, len(files))
	for _, f := range files {
		out = append(out, FileExplanation{Path: f.Path, Purpose: f.Kind})
	}
	return &Result{
		Status:         "preview",
		Summary:        fmt.Sprintf("%d file(s) would be explained.", len(files)),
		Files:          out,
		Relationships:  a.relationshipsForOutput(),
		Warnings:       a.warnings,
		GraphPath:      a.args.GraphPath,
		FilesExplained: int64(len(files)),
		Elapsed:        time.Since(start).Round(time.Second).String(),
	}
}

func (a *Agent) explainFile(ctx context.Context, path string) (FileExplanation, error) {
	content, err := readFileForPrompt(a.args.RepoDir, a.args.Ref, path)
	if err != nil {
		return FileExplanation{}, err
	}
	symbols := []string(nil)
	graphContext := ""
	if a.graph != nil {
		symbols = a.graph.SymbolsForFile(path)
		graphContext = a.graph.Neighbors(path, 30)
	}
	if len(content) > maxInlineFileBytes {
		content = content[:maxInlineFileBytes] + "\n\n[truncated]"
	}
	prompt := buildFilePrompt(path, content, symbols, graphContext, a.args.Background)
	return a.runExplainLoop(ctx, path, prompt)
}

func (a *Agent) explainArchitecture(ctx context.Context, files []FileExplanation) (string, string, []string, error) {
	prompt := buildArchitecturePrompt(files, a.graph, a.args.Background)
	result, err := a.runExplainLoop(ctx, "architecture", prompt)
	if err != nil {
		return "", "", nil, err
	}
	summary := result.Purpose
	if summary == "" {
		summary = "Tutor-mode connected architecture explanation."
	}
	return result.Explanation, summary, result.Gotchas, nil
}

func (a *Agent) runExplainLoop(ctx context.Context, path, userPrompt string) (FileExplanation, error) {
	if a.args.LLMClient == nil {
		return fallbackFileExplanation(path, userPrompt), nil
	}
	collector := &ResultCollector{}
	registry := a.buildToolRegistry(collector, path)
	messages := []llm.Message{
		llm.NewTextMessage("system", explainSystemPrompt()),
		llm.NewTextMessage("user", userPrompt),
	}
	toolDefs := ToolDefs(a.graph != nil)
	fs := a.session.GetOrCreateFileSession(path)
	for i := 0; i < a.args.MaxTools; i++ {
		rec := fs.AppendTaskRecord(session.MainTask, append([]llm.Message(nil), messages...))
		start := time.Now()
		resp, err := a.args.LLMClient.CompletionsWithCtx(ctx, llm.ChatRequest{
			Model:     a.args.Model,
			Messages:  messages,
			Tools:     toolDefs,
			MaxTokens: 12000,
		})
		if err != nil {
			rec.SetError(err, time.Since(start))
			return FileExplanation{}, err
		}
		rec.SetResponse(resp, time.Since(start))
		a.recordUsage(resp)
		content := resp.Content()
		calls := resp.ToolCalls()
		if len(calls) == 0 {
			if content != "" {
				return FileExplanation{Path: path, Explanation: content}, nil
			}
			messages = append(messages, llm.NewTextMessage("user", "Use explain_result to submit the tutor explanation."))
			continue
		}
		var results []tool.ToolCallResult
		for _, call := range calls {
			result := executeExplainTool(registry, call)
			if rec != nil {
				rec.AddToolResult(call.Function.Name, call.Function.Arguments, result)
			}
			results = append(results, tool.ToolCallResult{
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Result:     result,
			})
		}
		if explained, _, ok := collector.Snapshot(); ok {
			return explained, nil
		}
		messages = append(messages, llm.NewToolCallMessage(content, calls))
		for _, r := range results {
			messages = append(messages, llm.NewToolResultMessage(r.ToolCallID, r.Result))
		}
	}
	if explained, _, ok := collector.Snapshot(); ok {
		return explained, nil
	}
	return FileExplanation{}, fmt.Errorf("max explain tool rounds reached for %s", path)
}

func (a *Agent) buildToolRegistry(collector *ResultCollector, path string) tool.Registry {
	mode := tool.ModeWorkspace
	ref := ""
	if a.args.Ref != "" {
		mode = tool.ModeCommit
		ref = a.args.Ref
	}
	fr := &tool.FileReader{RepoDir: a.args.RepoDir, Mode: mode, Ref: ref}
	reg := tool.NewRegistry()
	reg.Register(tool.NewFileRead(fr))
	reg.Register(tool.NewFileFind(fr))
	reg.Register(tool.NewCodeSearch(fr))
	reg.Register(&ExplainResultProvider{Collector: collector, Path: path})
	if a.graph != nil {
		reg.Register(&GraphQueryProvider{Graph: a.graph})
		reg.Register(&GraphNeighborsProvider{Graph: a.graph})
	}
	return reg
}

func executeExplainTool(registry tool.Registry, call llm.ToolCall) string {
	provider, ok := registry[call.Function.Name]
	if !ok {
		return tool.NotAvailableMsg
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("Error parsing tool arguments: %v", err)
	}
	result, err := provider.Execute(args)
	if err != nil {
		return fmt.Sprintf("Error executing tool %s: %v", call.Function.Name, err)
	}
	if result == "" {
		return "Tool returned no result."
	}
	return result
}

func (a *Agent) recordUsage(resp *llm.ChatResponse) {
	if resp == nil || resp.Usage == nil {
		return
	}
	atomic.AddInt64(&a.totalTokens, int64(resp.Usage.TotalTokens))
	atomic.AddInt64(&a.totalInputTokens, int64(resp.Usage.PromptTokens+resp.Usage.CacheReadTokens))
	atomic.AddInt64(&a.totalOutputTokens, int64(resp.Usage.CompletionTokens+resp.Usage.CacheWriteTokens))
}

func (a *Agent) relationshipsForOutput() []Relationship {
	if a.graph == nil {
		return nil
	}
	limit := 200
	if len(a.graph.Relationships) < limit {
		limit = len(a.graph.Relationships)
	}
	out := make([]Relationship, limit)
	copy(out, a.graph.Relationships[:limit])
	return out
}

func readFileForPrompt(repoDir, ref, path string) (string, error) {
	if ref != "" {
		cmd := exec.Command("git", "show", ref+":"+path)
		cmd.Dir = repoDir
		out, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("git show %s:%s: %w", ref, path, err)
		}
		return string(out), nil
	}
	data, err := os.ReadFile(filepath.Join(repoDir, path))
	if err != nil {
		return "", fmt.Errorf("read file %s: %w", path, err)
	}
	return string(data), nil
}

func detectGitBranch(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func fallbackFileExplanation(path, prompt string) FileExplanation {
	return FileExplanation{
		Path:        path,
		Purpose:     "Deterministic fallback explanation",
		Explanation: "LLM client is not configured; tutor mode selected this file and prepared explanation context.\n\n" + prompt,
	}
}

func fallbackArchitecture(files []FileExplanation, graph *Graph) string {
	var sb strings.Builder
	sb.WriteString("## Connected Architecture\n\n")
	sb.WriteString("The project is organized around these explained files:\n\n")
	for _, f := range files {
		sb.WriteString("- `" + f.Path + "`")
		if f.Purpose != "" {
			sb.WriteString(": " + f.Purpose)
		}
		sb.WriteString("\n")
	}
	if graph != nil && len(graph.Relationships) > 0 {
		sb.WriteString("\nGraph relationships are available in the JSON output for deeper traversal.\n")
	}
	return sb.String()
}

func buildFilePrompt(path, content string, symbols []string, graphContext, background string) string {
	var sb strings.Builder
	sb.WriteString("Explain this file deeply for a developer who needs to maintain it.\n\n")
	if background != "" {
		sb.WriteString("Learning context: " + background + "\n\n")
	}
	sb.WriteString("File path: " + path + "\n")
	if len(symbols) > 0 {
		sb.WriteString("Graph symbols: " + strings.Join(symbols, ", ") + "\n")
	}
	if graphContext != "" && graphContext != "No graph neighbors found." {
		sb.WriteString("\nGraph relationship context:\n" + graphContext + "\n")
	}
	sb.WriteString("\nFile content:\n```text\n" + content + "\n```\n\n")
	sb.WriteString("Use explain_result with purpose, explanation, symbols, dependencies, dependents, and gotchas. Do not propose code changes.")
	return sb.String()
}

func buildArchitecturePrompt(files []FileExplanation, graph *Graph, background string) string {
	var sb strings.Builder
	sb.WriteString("Create a connected architecture explanation from these per-file tutor notes.\n")
	if background != "" {
		sb.WriteString("Learning context: " + background + "\n")
	}
	sb.WriteString("\nPer-file notes:\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("\n## %s\nPurpose: %s\nSymbols: %s\nDependencies: %s\nDependents: %s\nExplanation:\n%s\n",
			f.Path, f.Purpose, strings.Join(f.Symbols, ", "), strings.Join(f.Dependencies, ", "), strings.Join(f.Dependents, ", "), f.Explanation))
	}
	if graph != nil {
		sb.WriteString("\nGraph relationships:\n")
		limit := 80
		if len(graph.Relationships) < limit {
			limit = len(graph.Relationships)
		}
		for _, r := range graph.Relationships[:limit] {
			sb.WriteString(fmt.Sprintf("- %s --%s--> %s\n", r.Source, r.Type, r.Target))
		}
	}
	sb.WriteString("\nUse explain_result. Put the connected architecture in explanation and important flows or study path items in gotchas. Do not propose code changes.")
	return sb.String()
}

func explainSystemPrompt() string {
	return `You are a read-only code tutor. Your job is to help a developer deeply understand code, not critique or modify it.

Rules:
- Never suggest applying fixes unless the user explicitly asks outside tutor mode.
- Prefer concrete explanations grounded in files, symbols, data flow, and runtime behavior.
- Explain purpose, dependencies, dependents, important functions/types, and gotchas.
- Use available tools to inspect project context when needed.
- Submit final output with explain_result.`
}

func printProgress(format string, msg string) {
	if format == "json" {
		return
	}
	fmt.Fprintln(stdout.Writer(), msg)
}
