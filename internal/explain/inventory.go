package explain

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var explainSkipPrefixes = []string{
	".git/",
	"node_modules/",
	"vendor/",
	"target/",
	"dist/",
	"build/",
	".cache/",
	"graphify-out/",
	"explain-out/",
}

var explainSkipSuffixes = []string{
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".tar", ".gz",
	".lock", ".sum",
}

// Inventory lists files for tutor mode from git, optionally scoped to targets.
func Inventory(repoDir, ref string, targets []string, maxFiles int) ([]InventoryFile, error) {
	files, err := listRepoFiles(repoDir, ref)
	if err != nil {
		return nil, err
	}
	targetSet := normalizeTargets(targets)
	var out []InventoryFile
	for _, path := range files {
		if shouldSkipExplainPath(path) {
			continue
		}
		if len(targetSet) > 0 && !matchesTarget(path, targetSet) {
			continue
		}
		out = append(out, InventoryFile{Path: path, Kind: classifyPath(path)})
		if maxFiles > 0 && len(out) >= maxFiles {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func listRepoFiles(repoDir, ref string) ([]string, error) {
	var cmd *exec.Cmd
	if ref != "" {
		cmd = exec.Command("git", "ls-tree", "-r", "--name-only", ref)
	} else {
		cmd = exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	}
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list repository files: %w", err)
	}
	lines := bytes.Split(bytes.TrimRight(output, "\n"), []byte{'\n'})
	var files []string
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(line)))
	}
	return files, nil
}

func normalizeTargets(targets []string) []string {
	var out []string
	for _, target := range targets {
		target = strings.TrimSpace(filepath.ToSlash(target))
		target = strings.TrimPrefix(target, "./")
		if target != "" {
			out = append(out, target)
		}
	}
	return out
}

func matchesTarget(path string, targets []string) bool {
	for _, target := range targets {
		if path == target || strings.HasPrefix(path, strings.TrimSuffix(target, "/")+"/") {
			return true
		}
	}
	return false
}

func shouldSkipExplainPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	for _, prefix := range explainSkipPrefixes {
		if lower == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	for _, suffix := range explainSkipSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func classifyPath(path string) string {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(base))
	switch {
	case base == "README.md" || ext == ".md" || ext == ".mdx":
		return "docs"
	case base == "go.mod" || base == "package.json" || strings.Contains(base, "config") || ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml":
		return "config"
	case strings.Contains(base, "test") || strings.Contains(base, "spec"):
		return "test"
	default:
		return "source"
	}
}

// ExistingTargets validates that user-provided targets match at least one file.
func ExistingTargets(repoDir string, targets []string) []string {
	var out []string
	for _, target := range normalizeTargets(targets) {
		if _, err := os.Stat(filepath.Join(repoDir, target)); err == nil {
			out = append(out, target)
		}
	}
	return out
}
