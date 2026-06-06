package explain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeArtifactChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// WriteArtifacts writes the full explanation to Markdown and JSON files.
func WriteArtifacts(repoDir, outDir string, result *Result) error {
	if !filepath.IsAbs(outDir) {
		outDir = filepath.Join(repoDir, outDir)
	}
	filesDir := filepath.Join(outDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "README.md"), []byte(renderArtifactREADME(result)), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "architecture.md"), []byte(renderArchitecture(result)), 0644); err != nil {
		return err
	}
	for _, file := range result.Files {
		name := sanitizeArtifactName(file.Path) + ".md"
		if err := os.WriteFile(filepath.Join(filesDir, name), []byte(renderFileArtifact(file)), 0644); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "index.json"), data, 0644)
}

func renderArtifactREADME(result *Result) string {
	var sb strings.Builder
	sb.WriteString("# Code Explanation\n\n")
	sb.WriteString(result.Summary + "\n\n")
	sb.WriteString("## Files\n\n")
	for _, file := range result.Files {
		sb.WriteString(fmt.Sprintf("- [%s](files/%s.md)\n", file.Path, sanitizeArtifactName(file.Path)))
	}
	sb.WriteString("\nSee [architecture.md](architecture.md) for the connected explanation.\n")
	return sb.String()
}

func renderArchitecture(result *Result) string {
	var sb strings.Builder
	sb.WriteString("# Connected Architecture\n\n")
	sb.WriteString(result.Architecture + "\n")
	if len(result.Flows) > 0 {
		sb.WriteString("\n## Flows And Study Notes\n\n")
		for _, flow := range result.Flows {
			sb.WriteString("- " + flow + "\n")
		}
	}
	return sb.String()
}

func renderFileArtifact(file FileExplanation) string {
	var sb strings.Builder
	sb.WriteString("# " + file.Path + "\n\n")
	if file.Purpose != "" {
		sb.WriteString("## Purpose\n\n" + file.Purpose + "\n\n")
	}
	sb.WriteString("## Explanation\n\n" + file.Explanation + "\n")
	writeList := func(title string, values []string) {
		if len(values) == 0 {
			return
		}
		sb.WriteString("\n## " + title + "\n\n")
		for _, value := range values {
			sb.WriteString("- " + value + "\n")
		}
	}
	writeList("Symbols", file.Symbols)
	writeList("Dependencies", file.Dependencies)
	writeList("Dependents", file.Dependents)
	writeList("Gotchas", file.Gotchas)
	return sb.String()
}

func sanitizeArtifactName(path string) string {
	name := unsafeArtifactChars.ReplaceAllString(filepath.ToSlash(path), "__")
	return strings.Trim(name, "_")
}
