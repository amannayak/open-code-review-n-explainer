package llm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ResolvedEndpoint holds the resolved LLM endpoint configuration.
type ResolvedEndpoint struct {
	URL       string
	Token     string
	Model     string
	Protocol  string         // "anthropic" or "openai"
	Source    string         // human-readable config source label
	ExtraBody map[string]any // vendor-specific request body fields
}

// Environment variable names for OCR-specific configuration.
const (
	envOCRLLMURL            = "OCR_LLM_URL"
	envOCRLLMToken          = "OCR_LLM_TOKEN"
	envOCRLLMModel          = "OCR_LLM_MODEL"
	envOCRUseAnthropic      = "OCR_USE_ANTHROPIC"
	envOCRAllowLocalNoToken = "OCR_ALLOW_LOCAL_LLM_NO_TOKEN"
)

// Environment variable names from Claude Code configuration.
const (
	envCCBaseURL = "ANTHROPIC_BASE_URL"
	envCCToken   = "ANTHROPIC_AUTH_TOKEN"
	envCCModel   = "ANTHROPIC_MODEL"
)

// ResolveEndpoint reads from 4 strategy sources in priority order.
// Each strategy requires all three fields (URL, Token, Model) to be non-empty.
// Returns the first valid strategy's result.
func ResolveEndpoint(configPath string) (ResolvedEndpoint, error) {
	strategies := []struct {
		name string
		fn   func() (ResolvedEndpoint, bool, error)
	}{
		{"OCR config file", func() (ResolvedEndpoint, bool, error) { return tryOCRConfig(configPath) }},
		{"OCR environment", tryOCREnv},
		{"Claude Code environment", tryCCEnv},
		{"Shell rc file", tryShellRC},
	}

	for _, s := range strategies {
		ep, ok, err := s.fn()
		if err != nil {
			return ResolvedEndpoint{}, fmt.Errorf("resolve %s: %w", s.name, err)
		}
		if ok && ep.URL != "" && ep.Model != "" && (ep.Token != "" || ep.AllowLocalNoToken()) {
			ep.Source = s.name
			ep.Model = stripModelSuffix(ep.Model)
			return ep, nil
		}
	}

	return ResolvedEndpoint{}, fmt.Errorf("no valid LLM endpoint configured; one of OCR_LLM_URL/OCR_LLM_TOKEN/OCR_LLM_MODEL, ~/.opencodereview/config.json, or ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN/ANTHROPIC_MODEL must be set; for local OpenAI-compatible endpoints without a token, set llm.allow_local_no_token=true or OCR_ALLOW_LOCAL_LLM_NO_TOKEN=true")
}

// tryOCREnv reads OCR-specific environment variables.
func tryOCREnv() (ResolvedEndpoint, bool, error) {
	url := os.Getenv(envOCRLLMURL)
	token := os.Getenv(envOCRLLMToken)
	model := os.Getenv(envOCRLLMModel)
	if url == "" || model == "" {
		return ResolvedEndpoint{}, false, nil
	}
	if token == "" && !allowLocalNoToken(url, os.Getenv(envOCRAllowLocalNoToken)) {
		return ResolvedEndpoint{}, false, nil
	}

	useAnthropic := true // default true
	if v := os.Getenv(envOCRUseAnthropic); v != "" {
		lower := strings.ToLower(v)
		useAnthropic = lower == "true" || lower == "1" || lower == "yes"
	}

	protocol := "anthropic"
	if !useAnthropic {
		protocol = "openai"
	}

	return ResolvedEndpoint{URL: url, Token: token, Model: model, Protocol: protocol, Source: "OCR environment"}, true, nil
}

// llmFileConfig represents the llm section in config.json.
type llmFileConfig struct {
	URL               string         `json:"url,omitempty"`
	AuthToken         string         `json:"auth_token,omitempty"`
	Model             string         `json:"model,omitempty"`
	UseAnthropic      *bool          `json:"use_anthropic,omitempty"` // pointer to distinguish unset from false
	ExtraBody         map[string]any `json:"extra_body,omitempty"`
	AllowLocalNoToken bool           `json:"allow_local_no_token,omitempty"`
}

type configFile struct {
	Llm llmFileConfig `json:"llm,omitempty"`
}

// tryOCRConfig reads the OCR config file.
func tryOCRConfig(path string) (ResolvedEndpoint, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ResolvedEndpoint{}, false, nil
		}
		return ResolvedEndpoint{}, false, err
	}

	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return ResolvedEndpoint{}, false, fmt.Errorf("parse config: %w", err)
	}

	if cfg.Llm.URL == "" || cfg.Llm.Model == "" {
		return ResolvedEndpoint{}, false, nil
	}
	if cfg.Llm.AuthToken == "" && !allowLocalNoTokenBool(cfg.Llm.URL, cfg.Llm.AllowLocalNoToken) {
		return ResolvedEndpoint{}, false, nil
	}

	useAnthropic := true // default true
	if cfg.Llm.UseAnthropic != nil {
		useAnthropic = *cfg.Llm.UseAnthropic
	}

	protocol := "anthropic"
	if !useAnthropic {
		protocol = "openai"
	}

	return ResolvedEndpoint{URL: cfg.Llm.URL, Token: cfg.Llm.AuthToken, Model: cfg.Llm.Model, Protocol: protocol, Source: "OCR config file", ExtraBody: cfg.Llm.ExtraBody}, true, nil
}

// AllowLocalNoToken reports whether this endpoint may omit auth because it is explicitly local.
func (r ResolvedEndpoint) AllowLocalNoToken() bool {
	return r.Token == "" && isLocalLLMURL(r.URL)
}

func allowLocalNoToken(rawURL, flag string) bool {
	lower := strings.ToLower(strings.TrimSpace(flag))
	return allowLocalNoTokenBool(rawURL, lower == "true" || lower == "1" || lower == "yes")
}

func allowLocalNoTokenBool(rawURL string, allowed bool) bool {
	return allowed && isLocalLLMURL(rawURL)
}

func isLocalLLMURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasPrefix(host, "10.") ||
		strings.HasPrefix(host, "192.168.") ||
		isPrivate172(host)
}

func isPrivate172(host string) bool {
	if !strings.HasPrefix(host, "172.") {
		return false
	}
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return false
	}
	switch parts[1] {
	case "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31":
		return true
	default:
		return false
	}
}

// tryCCEnv reads Claude Code environment variables.
func tryCCEnv() (ResolvedEndpoint, bool, error) {
	baseURL := os.Getenv(envCCBaseURL)
	token := os.Getenv(envCCToken)
	model := os.Getenv(envCCModel)
	if baseURL == "" || token == "" || model == "" {
		return ResolvedEndpoint{}, false, nil
	}

	url := ensureMessagesSuffix(baseURL)

	return ResolvedEndpoint{URL: url, Token: token, Model: model, Protocol: "anthropic", Source: "Claude Code environment"}, true, nil
}

// tryShellRC parses ~/.zshrc and ~/.bashrc for ANTHROPIC_* exports.
func tryShellRC() (ResolvedEndpoint, bool, error) {
	files := shellRCFiles()
	for _, f := range files {
		ep, ok, err := parseShellRC(f)
		if err != nil || ok {
			return ep, ok, err
		}
	}
	return ResolvedEndpoint{}, false, nil
}

func shellRCFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
	}
	var valid []string
	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			valid = append(valid, f)
		}
	}
	return valid
}

var exportRe = regexp.MustCompile(`^export\s+(ANTHROPIC_\w+)\s*=\s*(?:"([^"]*)"|'([^']*)'|(.+))\s*$`)

var modelSuffixRe = regexp.MustCompile(`\[\d+m\]$`)

func stripModelSuffix(model string) string {
	return modelSuffixRe.ReplaceAllString(model, "")
}

func parseShellRC(path string) (ResolvedEndpoint, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ResolvedEndpoint{}, false, nil
	}

	var baseURL, token, model string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		matches := exportRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		key := matches[1]
		value := matches[2]
		if value == "" {
			value = matches[3]
		}
		if value == "" {
			value = matches[4]
		}
		value = strings.TrimSpace(value)

		switch key {
		case "ANTHROPIC_BASE_URL":
			baseURL = value
		case "ANTHROPIC_AUTH_TOKEN":
			token = value
		case "ANTHROPIC_MODEL":
			model = value
		}
	}

	if baseURL == "" || token == "" || model == "" {
		return ResolvedEndpoint{}, false, nil
	}

	url := ensureMessagesSuffix(baseURL)

	return ResolvedEndpoint{URL: url, Token: token, Model: model, Protocol: "anthropic", Source: "Shell rc file"}, true, nil
}

// ensureMessagesSuffix appends /v1/messages to base URLs that lack a versioned path.
func ensureMessagesSuffix(rawURL string) string {
	u := strings.TrimRight(rawURL, "/")
	if strings.Contains(u, "/v1/") {
		// Already has versioned path — don't modify.
		return rawURL
	}
	return u + "/v1/messages"
}
