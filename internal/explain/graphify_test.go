package explain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGraph_TolerantShape(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	data := `{
  "graph": {
    "nodes": [
      {"id":"file:main.go","label":"main.go","type":"file","path":"main.go","community":"app","confidence":"EXTRACTED"},
      {"id":"sym:main","label":"main","type":"function","file_path":"main.go","confidence":"EXTRACTED"}
    ],
    "edges": [
      {"source":"sym:main","target":"file:main.go","type":"defined_in","confidence":"EXTRACTED"}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	graph, err := LoadGraph(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != 2 {
		t.Fatalf("nodes = %d", len(graph.Nodes))
	}
	if got := graph.FilePaths(); len(got) != 1 || got[0] != "main.go" {
		t.Fatalf("FilePaths() = %v", got)
	}
	if got := graph.SymbolsForFile("main.go"); len(got) != 1 || got[0] != "main" {
		t.Fatalf("SymbolsForFile() = %v", got)
	}
	if got := graph.Neighbors("main.go", 5); !strings.Contains(got, "defined_in") {
		t.Fatalf("Neighbors() = %q", got)
	}
}

func TestLoadGraph_NoNodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(path, []byte(`{"edges":[]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraph(path); err == nil {
		t.Fatal("expected no nodes error")
	}
}
