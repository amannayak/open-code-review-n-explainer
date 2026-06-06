package explain

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/open-code-review/open-code-review/internal/llm"
	"github.com/open-code-review/open-code-review/internal/tool"
)

// ResultCollector stores the latest explain_result payload.
type ResultCollector struct {
	mu     sync.Mutex
	Result FileExplanation
	Raw    string
	Seen   bool
}

func (c *ResultCollector) Set(result FileExplanation, raw string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Result = result
	c.Raw = raw
	c.Seen = true
}

func (c *ResultCollector) Snapshot() (FileExplanation, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Result, c.Raw, c.Seen
}

// ExplainResultProvider lets the model submit final tutor content.
type ExplainResultProvider struct {
	Collector *ResultCollector
	Path      string
}

func (p *ExplainResultProvider) Tool() tool.Tool { return tool.ExplainResult }

func (p *ExplainResultProvider) Execute(args map[string]any) (string, error) {
	if p.Collector == nil {
		return "Error: explain result collector is not configured", nil
	}
	result := FileExplanation{
		Path:         firstArgString(args, "path"),
		Purpose:      firstArgString(args, "purpose"),
		Explanation:  firstArgString(args, "explanation", "content"),
		Symbols:      stringSliceArg(args, "symbols"),
		Dependencies: stringSliceArg(args, "dependencies"),
		Dependents:   stringSliceArg(args, "dependents"),
		Gotchas:      stringSliceArg(args, "gotchas"),
	}
	if result.Path == "" {
		result.Path = p.Path
	}
	if result.Explanation == "" {
		raw, _ := json.Marshal(args)
		return fmt.Sprintf("Error: explanation is required. Got args: %s", string(raw)), nil
	}
	raw, _ := json.Marshal(args)
	p.Collector.Set(result, string(raw))
	return "Explanation captured.", nil
}

// GraphQueryProvider searches the normalized Graphify graph.
type GraphQueryProvider struct {
	Graph *Graph
}

func (p *GraphQueryProvider) Tool() tool.Tool { return tool.GraphQuery }

func (p *GraphQueryProvider) Execute(args map[string]any) (string, error) {
	if p.Graph == nil {
		return "No graph is loaded.", nil
	}
	query := firstArgString(args, "query", "search_text")
	limit := intArg(args, "limit", 20)
	return p.Graph.Search(query, limit), nil
}

// GraphNeighborsProvider returns graph edges around a file, symbol, or node id.
type GraphNeighborsProvider struct {
	Graph *Graph
}

func (p *GraphNeighborsProvider) Tool() tool.Tool { return tool.GraphNeighbors }

func (p *GraphNeighborsProvider) Execute(args map[string]any) (string, error) {
	if p.Graph == nil {
		return "No graph is loaded.", nil
	}
	id := firstArgString(args, "id", "path", "node")
	limit := intArg(args, "limit", 20)
	return p.Graph.Neighbors(id, limit), nil
}

// ToolDefs returns the explain-mode tool definitions. It deliberately excludes code_comment.
func ToolDefs(includeGraph bool) []llm.ToolDef {
	defs := []llm.ToolDef{
		functionDef("file_read", "Read a project file or line range for explanation context.", map[string]any{
			"file_path":  map[string]any{"type": "string", "description": "Relative path of the file to read."},
			"start_line": map[string]any{"type": "integer", "description": "Optional 1-based start line."},
			"end_line":   map[string]any{"type": "integer", "description": "Optional 1-based end line."},
		}, []string{"file_path"}),
		functionDef("file_find", "Find files by filename keyword when locating code to explain.", map[string]any{
			"query_name":     map[string]any{"type": "string", "description": "Filename keyword to search for."},
			"case_sensitive": map[string]any{"type": "boolean", "description": "Whether matching is case-sensitive."},
		}, []string{"query_name"}),
		functionDef("code_search", "Search project source text to understand symbols, call sites, and flows.", map[string]any{
			"search_text":     map[string]any{"type": "string", "description": "Text or regular expression to search for."},
			"file_patterns":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional git pathspec filters."},
			"case_sensitive":  map[string]any{"type": "boolean", "description": "Whether search is case-sensitive."},
			"use_perl_regexp": map[string]any{"type": "boolean", "description": "Use Perl-compatible regular expressions."},
		}, []string{"search_text"}),
		functionDef("explain_result", "Submit the final tutor explanation for the current file or project section.", map[string]any{
			"path":         map[string]any{"type": "string", "description": "File path or section name being explained."},
			"purpose":      map[string]any{"type": "string", "description": "Short purpose of the file or section."},
			"explanation":  map[string]any{"type": "string", "description": "Deep teaching-oriented explanation."},
			"symbols":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"dependencies": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"dependents":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"gotchas":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}, []string{"explanation"}),
	}
	if includeGraph {
		graphDefs := []llm.ToolDef{
			functionDef("graph_query", "Search the project knowledge graph for files, symbols, concepts, or relationships.", map[string]any{
				"query": map[string]any{"type": "string", "description": "Search text."},
				"limit": map[string]any{"type": "integer", "description": "Maximum matches to return."},
			}, []string{"query"}),
			functionDef("graph_neighbors", "Get graph relationships around a file path, symbol, or node id.", map[string]any{
				"id":    map[string]any{"type": "string", "description": "File path, symbol, or graph node id."},
				"limit": map[string]any{"type": "integer", "description": "Maximum relationships to return."},
			}, []string{"id"}),
		}
		defs = append(graphDefs, defs...)
	}
	return defs
}

func functionDef(name, description string, properties map[string]any, required []string) llm.ToolDef {
	req := make([]any, 0, len(required))
	for _, item := range required {
		req = append(req, item)
	}
	return llm.ToolDef{
		Type: "function",
		Function: llm.FunctionDef{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": properties,
				"required":   req,
			},
		},
	}
}

func firstArgString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := args[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func stringSliceArg(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

func intArg(args map[string]any, key string, fallback int) int {
	switch v := args[key].(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return fallback
}
