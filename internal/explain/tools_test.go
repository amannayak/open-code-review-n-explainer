package explain

import (
	"strings"
	"testing"
)

func TestToolDefs_ExcludeReviewTools(t *testing.T) {
	defs := ToolDefs(true)
	var names []string
	for _, def := range defs {
		names = append(names, def.Function.Name)
	}
	joined := strings.Join(names, ",")
	if strings.Contains(joined, "code_comment") {
		t.Fatalf("explain tool defs should not include code_comment: %v", names)
	}
	for _, want := range []string{"file_read", "file_find", "code_search", "graph_query", "graph_neighbors", "explain_result"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
}

func TestExplainResultProvider(t *testing.T) {
	collector := &ResultCollector{}
	provider := &ExplainResultProvider{Collector: collector, Path: "main.go"}
	result, err := provider.Execute(map[string]any{
		"purpose":     "entrypoint",
		"explanation": "starts the app",
		"symbols":     []any{"main"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Explanation captured." {
		t.Fatalf("result = %q", result)
	}
	explained, _, ok := collector.Snapshot()
	if !ok {
		t.Fatal("collector did not capture result")
	}
	if explained.Path != "main.go" || explained.Symbols[0] != "main" {
		t.Fatalf("captured = %+v", explained)
	}
}
