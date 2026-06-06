package explain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-code-review/open-code-review/internal/llm"
)

type fakeLLM struct {
	responses []*llm.ChatResponse
	idx       int
}

func (f *fakeLLM) Completions(req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f.next(), nil
}

func (f *fakeLLM) CompletionsWithCtx(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return f.next(), nil
}

func (f *fakeLLM) StreamCompletion(req llm.ChatRequest, cb func(chunk []byte) error) error {
	return nil
}

func (f *fakeLLM) next() *llm.ChatResponse {
	if f.idx >= len(f.responses) {
		return f.responses[len(f.responses)-1]
	}
	resp := f.responses[f.idx]
	f.idx++
	return resp
}

func TestAgentRun_TargetedWithMockLLM(t *testing.T) {
	dir := setupExplainRepo(t)
	content1 := "file explanation"
	content2 := "architecture explanation"
	client := &fakeLLM{responses: []*llm.ChatResponse{
		toolCallResponse("call1", "explain_result", `{"path":"src/main.go","purpose":"entrypoint","explanation":"`+content1+`","symbols":["main"]}`),
		toolCallResponse("call2", "explain_result", `{"path":"architecture","purpose":"summary","explanation":"`+content2+`","gotchas":["startup flow"]}`),
	}}

	result, err := New(Args{
		RepoDir:   dir,
		Targets:   []string{"src/main.go"},
		LLMClient: client,
		Model:     "fake",
		MaxFiles:  5,
		MaxTools:  3,
	}).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files = %d", len(result.Files))
	}
	if result.Files[0].Explanation != content1 {
		t.Fatalf("file explanation = %q", result.Files[0].Explanation)
	}
	if result.Architecture != content2 {
		t.Fatalf("architecture = %q", result.Architecture)
	}
}

func TestWriteArtifacts(t *testing.T) {
	dir := t.TempDir()
	result := &Result{
		Summary:      "summary",
		Architecture: "architecture",
		Files: []FileExplanation{{
			Path:        "src/main.go",
			Purpose:     "entrypoint",
			Explanation: "starts the app",
		}},
	}
	if err := WriteArtifacts(dir, "explain-out", result); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"explain-out/README.md", "explain-out/architecture.md", "explain-out/index.json"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}
	files, err := os.ReadDir(filepath.Join(dir, "explain-out", "files"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || !strings.HasSuffix(files[0].Name(), ".md") {
		t.Fatalf("artifact files = %v", files)
	}
}

func toolCallResponse(id, name, args string) *llm.ChatResponse {
	return &llm.ChatResponse{
		Model: "fake",
		Choices: []llm.Choice{{
			Message: llm.ResponseMessage{
				ToolCalls: []llm.ToolCall{{
					ID:   id,
					Type: "function",
					Function: llm.FunctionCall{
						Name:      name,
						Arguments: args,
					},
				}},
			},
		}},
	}
}
