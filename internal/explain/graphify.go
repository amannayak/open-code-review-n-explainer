package explain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Graph is the normalized subset of graphify-out/graph.json used by tutor mode.
type Graph struct {
	Nodes         []GraphNode
	Relationships []Relationship
}

// GraphNode is a tolerant representation of a Graphify node.
type GraphNode struct {
	ID         string
	Label      string
	Type       string
	Path       string
	Community  string
	Confidence string
	SourceType string
}

// LoadGraph reads graphify-out/graph.json and normalizes common graph JSON shapes.
func LoadGraph(path string) (*Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read graph %s: %w", path, err)
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse graph %s: %w", path, err)
	}
	g := &Graph{}
	g.Nodes = extractNodes(root)
	g.Relationships = extractRelationships(root)
	if len(g.Nodes) == 0 {
		return nil, fmt.Errorf("graph %s contains no recognizable nodes", path)
	}
	return g, nil
}

// DefaultGraphPath returns the conventional Graphify output path for a repo.
func DefaultGraphPath(repoDir string) string {
	return filepath.Join(repoDir, "graphify-out", "graph.json")
}

// FilePaths returns sorted file paths discovered in the graph.
func (g *Graph) FilePaths() []string {
	seen := make(map[string]bool)
	for _, n := range g.Nodes {
		if n.Path == "" {
			continue
		}
		if looksLikeFileNode(n) {
			seen[n.Path] = true
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

// SymbolsForFile returns symbol labels associated with a file path.
func (g *Graph) SymbolsForFile(path string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, n := range g.Nodes {
		if n.Path != path || n.Label == "" || looksLikeFileNode(n) {
			continue
		}
		if !seen[n.Label] {
			seen[n.Label] = true
			out = append(out, n.Label)
		}
	}
	sort.Strings(out)
	return out
}

// RelationshipsFor returns relationships connected to a path or symbol.
func (g *Graph) RelationshipsFor(idOrPath string, limit int) []Relationship {
	if limit <= 0 {
		limit = 20
	}
	ids := g.nodeIDsFor(idOrPath)
	out := make([]Relationship, 0)
	for _, rel := range g.Relationships {
		if ids[rel.Source] || ids[rel.Target] || rel.Source == idOrPath || rel.Target == idOrPath {
			out = append(out, rel)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

// Search returns compact node and relationship matches for graph_query.
func (g *Graph) Search(query string, limit int) string {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return "No query provided."
	}
	if limit <= 0 {
		limit = 20
	}
	var sb strings.Builder
	count := 0
	for _, n := range g.Nodes {
		haystack := strings.ToLower(strings.Join([]string{n.ID, n.Label, n.Type, n.Path, n.Community}, " "))
		if strings.Contains(haystack, query) {
			sb.WriteString(fmt.Sprintf("NODE id=%s label=%s type=%s path=%s community=%s confidence=%s\n",
				n.ID, n.Label, n.Type, n.Path, n.Community, n.Confidence))
			count++
			if count >= limit {
				break
			}
		}
	}
	for _, r := range g.Relationships {
		haystack := strings.ToLower(strings.Join([]string{r.Source, r.Target, r.Type, r.Confidence, r.SourceType}, " "))
		if strings.Contains(haystack, query) {
			sb.WriteString(fmt.Sprintf("EDGE source=%s target=%s type=%s confidence=%s source_type=%s\n",
				r.Source, r.Target, r.Type, r.Confidence, r.SourceType))
			count++
			if count >= limit {
				break
			}
		}
	}
	if count == 0 {
		return "No graph matches found."
	}
	return sb.String()
}

// Neighbors returns compact relationship context for graph_neighbors.
func (g *Graph) Neighbors(idOrPath string, limit int) string {
	rels := g.RelationshipsFor(idOrPath, limit)
	if len(rels) == 0 {
		return "No graph neighbors found."
	}
	var sb strings.Builder
	for _, r := range rels {
		sb.WriteString(fmt.Sprintf("%s --%s--> %s", r.Source, r.Type, r.Target))
		if r.Confidence != "" {
			sb.WriteString(" confidence=" + r.Confidence)
		}
		if r.SourceType != "" {
			sb.WriteString(" source_type=" + r.SourceType)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func (g *Graph) nodeIDsFor(idOrPath string) map[string]bool {
	ids := map[string]bool{idOrPath: true}
	for _, n := range g.Nodes {
		if n.ID == idOrPath || n.Path == idOrPath || n.Label == idOrPath {
			ids[n.ID] = true
			if n.Path != "" {
				ids[n.Path] = true
			}
			if n.Label != "" {
				ids[n.Label] = true
			}
		}
	}
	return ids
}

func looksLikeFileNode(n GraphNode) bool {
	t := strings.ToLower(n.Type)
	if strings.Contains(t, "file") || strings.Contains(t, "document") || strings.Contains(t, "source") {
		return true
	}
	return n.Path != "" && (n.Label == "" || n.Label == n.Path || strings.HasSuffix(n.Path, n.Label))
}

func extractNodes(root any) []GraphNode {
	raw := findFirstArray(root, "nodes", "vertices")
	var out []GraphNode
	for _, item := range raw {
		obj := asObject(item)
		if len(obj) == 0 {
			continue
		}
		// Cytoscape-style nodes often wrap fields under data.
		if dataObj := asObject(obj["data"]); len(dataObj) > 0 {
			obj = mergeObject(dataObj, obj)
		}
		n := GraphNode{
			ID:         firstString(obj, "id", "key", "uuid"),
			Label:      firstString(obj, "label", "name", "title", "qualified_name"),
			Type:       firstString(obj, "type", "kind", "category", "node_type"),
			Path:       cleanPath(firstString(obj, "path", "file", "file_path", "filePath", "source_path", "sourcePath")),
			Community:  firstString(obj, "community", "cluster", "module", "group"),
			Confidence: firstString(obj, "confidence", "confidence_tag"),
			SourceType: firstString(obj, "source_type", "sourceType", "source"),
		}
		if n.Path == "" {
			n.Path = cleanPath(firstString(obj, "filepath", "filename"))
		}
		if n.ID == "" {
			n.ID = n.Path
		}
		if n.Label == "" {
			n.Label = n.ID
		}
		if n.ID != "" || n.Path != "" || n.Label != "" {
			out = append(out, n)
		}
	}
	return out
}

func extractRelationships(root any) []Relationship {
	raw := findFirstArray(root, "edges", "links", "relationships")
	var out []Relationship
	for _, item := range raw {
		obj := asObject(item)
		if len(obj) == 0 {
			continue
		}
		if dataObj := asObject(obj["data"]); len(dataObj) > 0 {
			obj = mergeObject(dataObj, obj)
		}
		r := Relationship{
			Source:     firstString(obj, "source", "from", "src", "source_id", "sourceId"),
			Target:     firstString(obj, "target", "to", "dst", "target_id", "targetId"),
			Type:       firstString(obj, "type", "kind", "label", "relation", "relationship"),
			Confidence: firstString(obj, "confidence", "confidence_tag"),
			SourceType: firstString(obj, "source_type", "sourceType"),
		}
		if r.Type == "" {
			r.Type = "related"
		}
		if r.Source != "" && r.Target != "" {
			out = append(out, r)
		}
	}
	return out
}

func findFirstArray(root any, keys ...string) []any {
	if arr, ok := root.([]any); ok {
		return arr
	}
	obj := asObject(root)
	for _, key := range keys {
		if arr, ok := obj[key].([]any); ok {
			return arr
		}
	}
	for _, nestedKey := range []string{"graph", "data", "elements"} {
		if nested := asObject(obj[nestedKey]); len(nested) > 0 {
			for _, key := range keys {
				if arr, ok := nested[key].([]any); ok {
					return arr
				}
			}
		}
	}
	return nil
}

func asObject(v any) map[string]any {
	if obj, ok := v.(map[string]any); ok {
		return obj
	}
	return nil
}

func mergeObject(primary, fallback map[string]any) map[string]any {
	out := make(map[string]any, len(primary)+len(fallback))
	for k, v := range fallback {
		out[k] = v
	}
	for k, v := range primary {
		out[k] = v
	}
	return out
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		switch v := obj[key].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case float64:
			return fmt.Sprintf("%.0f", v)
		case int:
			return fmt.Sprintf("%d", v)
		}
	}
	return ""
}

func cleanPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	return filepath.ToSlash(path)
}
