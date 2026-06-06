package explain

import "time"

// FileExplanation is the tutor-mode explanation for one repository file.
type FileExplanation struct {
	Path         string   `json:"path"`
	Purpose      string   `json:"purpose"`
	Explanation  string   `json:"explanation"`
	Symbols      []string `json:"symbols,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Dependents   []string `json:"dependents,omitempty"`
	Gotchas      []string `json:"gotchas,omitempty"`
}

// Relationship is a compact edge between files, symbols, or concepts.
type Relationship struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Type       string `json:"type"`
	Confidence string `json:"confidence,omitempty"`
	SourceType string `json:"source_type,omitempty"`
}

// Result is the structured output returned by tutor mode.
type Result struct {
	Status         string            `json:"status"`
	Summary        string            `json:"summary"`
	Architecture   string            `json:"architecture"`
	Files          []FileExplanation `json:"files"`
	Relationships  []Relationship    `json:"relationships,omitempty"`
	Flows          []string          `json:"flows,omitempty"`
	Warnings       []string          `json:"warnings,omitempty"`
	GraphPath      string            `json:"graph_path,omitempty"`
	FilesExplained int64             `json:"files_explained"`
	TotalTokens    int64             `json:"total_tokens"`
	InputTokens    int64             `json:"input_tokens"`
	OutputTokens   int64             `json:"output_tokens"`
	Elapsed        string            `json:"elapsed"`
}

// InventoryFile describes a file selected for explanation.
type InventoryFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

// RunStats holds execution counters used to complete Result metadata.
type RunStats struct {
	Started      time.Time
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}
