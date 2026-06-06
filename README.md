<p align="center">
  <img src="imgs/image.png" alt="open-code-review-n-explainer logo" width="240" height="240">
</p>
<p align="center"><strong>open-code-review-n-explainer</strong></p>
<p align="center">A fork of <a href="https://github.com/alibaba/open-code-review">alibaba/open-code-review</a> — adds <code>ocr explain</code>, a read-only code tutor mode.</p>
<p align="center">
  <a href="https://github.com/amannayak/open-code-review-n-explainer"><img alt="This fork" src="https://img.shields.io/badge/fork-amannayak%2Fopen--code--review--n--explainer-35BD5F?style=flat-square" /></a>
  <a href="https://github.com/alibaba/open-code-review"><img alt="Upstream" src="https://img.shields.io/badge/upstream-alibaba%2Fopen--code--review-blue?style=flat-square" /></a>
  <a href="https://github.com/alibaba/open-code-review/blob/main/LICENSE"><img alt="License" src="https://img.shields.io/github/license/alibaba/open-code-review?style=flat-square" /></a>
</p>

---

> **This is a fork.** The upstream project is [alibaba/open-code-review](https://github.com/alibaba/open-code-review). This fork keeps the full review pipeline intact and adds one new command: `ocr explain`.

## What's different from upstream?

The upstream `ocr review` reads Git diffs, sends changed files to a configurable LLM, and generates structured review comments with line-level precision. **This fork adds `ocr explain`** — a read-only tutor mode that deeply explains how existing code works, rather than critiquing changes to it.

### What's added in this fork

| Feature | Upstream | This fork |
|---------|----------|-----------|
| `ocr review` — AI code review on Git diffs | ✅ | ✅ |
| `ocr explain` — read-only tutor mode | ❌ | ✅ |
| Connected architecture synthesis | ❌ | ✅ |
| Graphify graph backend integration | ❌ | ✅ |
| Language-adaptive replies (responds in question's language) | ❌ | ✅ |

**Tutor mode is designed for increasingly agent-generated codebases** where the human maintainer needs deep understanding before changing anything. It explains files one by one, then synthesizes the connected architecture: entrypoints, modules, data flow, important symbols, dependencies, dependents, and gotchas.

Tutor mode never edits, stages, commits, or auto-fixes code.

---

## Quick Start

### 1. Install

```bash
git clone https://github.com/amannayak/open-code-review-n-explainer.git
cd open-code-review-n-explainer
make build
sudo cp dist/opencodereview /usr/local/bin/ocr
```

### 2. Configure LLM

```bash
# Anthropic
ocr config set llm.url https://api.anthropic.com/v1/messages
ocr config set llm.auth_token your-api-key
ocr config set llm.model claude-opus-4-6
ocr config set llm.use_anthropic true

# Local Ollama / LM Studio
ocr config set llm.url http://localhost:11434/v1/chat/completions
ocr config set llm.model gemma4:26b
ocr config set llm.use_anthropic false
ocr config set llm.allow_local_no_token true
```

Config is stored in `~/.opencodereview/config.json`.

### 3. Test connectivity

```bash
ocr llm test
```

The agent will reply in the same language you ask in. To pin a specific language:

```bash
ocr config set language English
```

---

## Usage

### Code Review

```bash
cd your-project

# Review staged, unstaged, and untracked changes
ocr review

# Review a branch range
ocr review --from main --to feature-branch

# Review a single commit
ocr review --commit abc123
```

### Code Explanation (Tutor Mode)

#### Without Graphify — explain specific files or directories

No extra dependencies needed. Pass any files or directories as arguments:

```bash
cd your-project

ocr explain internal/agent cmd/main.go
ocr explain src/auth src/api
```

The agent reads those files, explains each one, and synthesizes what it can from the files you gave it.

#### With Graphify — full connected architecture

For `--all-files` mode, tutor mode uses **Graphify** to build a dependency graph of the entire repository first. This lets it explain how every file connects to every other file — data flow, call chains, import relationships — not just individual files in isolation.

**Install Graphify once** (requires Python; `uv` or `pipx` must already be installed):

```bash
ocr setup explain
```

`ocr setup explain` tries `uv tool install graphifyy` first, then `pipx install graphifyy`. If neither is available, install manually:

```bash
# via uv (recommended)
uv tool install graphifyy

# via pipx
pipx install graphifyy

# verify
graphify --version
```

**Then run full-repo explanation:**

```bash
cd your-project

ocr explain --all-files --out explain-out
```

Artifacts (per-file notes, architecture synthesis, graph JSON) are written to `explain-out/`.

**Re-use an existing graph** (skips the graph build step):

```bash
ocr explain --all-files --graph graphify-out/graph.json
```

**Agent-friendly output:**

```bash
ocr explain --audience agent
```

---

## Integrate with Coding Agents

Both `ocr review` and `ocr explain` can be called from inside any AI coding agent (Claude Code, Cursor, etc.) as shell commands. The plugin and skill formats below register them as first-class slash commands.

### Claude Code — plugin

```bash
/plugin marketplace add amannayak/open-code-review-n-explainer
/plugin install open-code-review@open-code-review
```

This registers two slash commands in Claude Code:
- `/open-code-review:review` — runs `ocr review`, filters findings, and optionally applies fixes
- `/open-code-review:explain` — runs `ocr explain` in read-only tutor mode

### Claude Code — copy command files directly

No package manager needed. Copy the command files into your project or home directory:

```bash
mkdir -p .claude/commands
curl -o .claude/commands/open-code-review.md \
  https://raw.githubusercontent.com/amannayak/open-code-review-n-explainer/main/plugins/open-code-review/commands/review.md
curl -o .claude/commands/explain.md \
  https://raw.githubusercontent.com/amannayak/open-code-review-n-explainer/main/plugins/open-code-review/commands/explain.md
```

For user-level (all projects):

```bash
mkdir -p ~/.claude/commands
curl -o ~/.claude/commands/open-code-review.md \
  https://raw.githubusercontent.com/amannayak/open-code-review-n-explainer/main/plugins/open-code-review/commands/review.md
curl -o ~/.claude/commands/explain.md \
  https://raw.githubusercontent.com/amannayak/open-code-review-n-explainer/main/plugins/open-code-review/commands/explain.md
```

### Claude Code — skill

```bash
npx skills add amannayak/open-code-review-n-explainer --skill open-code-review-n-explainer
```

### Other agents (Cursor, generic harness)

The `ocr` binary is a plain CLI — any agent that can run shell commands can use it:

```bash
# Review current changes
ocr review --audience agent --format json

# Explain a file or directory
ocr explain --audience agent src/auth
```

Use `--audience agent` so output is compact and machine-readable. Use `--format json` on `review` for structured findings.

---

## Commands

| Command | Description |
|---------|-------------|
| `ocr review` | Start a code review |
| `ocr explain` | Explain code in tutor mode |
| `ocr setup explain` | Install Graphify for connected explanations |
| `ocr rules check <file>` | Preview which review rule applies to a file |
| `ocr config set <key> <value>` | Set a configuration value |
| `ocr llm test` | Test LLM connectivity |
| `ocr viewer` | Launch WebUI session viewer on `localhost:5483` |
| `ocr version` | Show version info |

### `ocr review` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | — | Source ref (e.g. `main`) |
| `--to` | — | Target ref |
| `--commit`, `-c` | — | Single commit |
| `--preview`, `-p` | `false` | Preview files without calling the LLM |
| `--format`, `-f` | `text` | `text` or `json` |
| `--concurrency` | `8` | Max concurrent file reviews |
| `--audience` | `human` | `human` or `agent` |
| `--rule` | — | Path to custom review rules JSON |

### `ocr explain` flags

| Flag | Default | Description |
|------|---------|-------------|
| `--all-files` | `false` | Explain all files (requires Graphify) |
| `--graph` | — | Path to an existing Graphify `graph.json` |
| `--out` | — | Write artifacts to this directory |
| `--audience` | `human` | `human` or `agent` |
| `--background` | — | Learning context passed to the tutor |
| `--max-files` | `50` | File limit per run |
| `--preview`, `-p` | `false` | Preview files without calling the LLM |

---

## Configuration Reference

Config file: `~/.opencodereview/config.json`

| Key | Type | Example |
|-----|------|---------|
| `llm.url` | string | `https://api.openai.com/v1/chat/completions` |
| `llm.auth_token` | string | `sk-xxxxxxx` |
| `llm.model` | string | `claude-opus-4-6` |
| `llm.use_anthropic` | boolean | `true` \| `false` |
| `llm.allow_local_no_token` | boolean | `true` for local endpoints |
| `language` | string | `English` — omit to reply in the question's language |
| `telemetry.enabled` | boolean | `true` \| `false` |

### Environment variables

| Variable | Purpose |
|----------|---------|
| `OCR_LLM_URL` | LLM endpoint URL |
| `OCR_LLM_TOKEN` | API key |
| `OCR_LLM_MODEL` | Model name |
| `OCR_USE_ANTHROPIC` | `true` = Anthropic, `false` = OpenAI |

---

## Review Rules

OCR resolves review rules via a four-layer priority chain:

| Priority | Source | Path |
|----------|--------|------|
| 1 | `--rule` flag | CLI explicit override |
| 2 | Project config | `<repoDir>/.opencodereview/rule.json` |
| 3 | Global config | `~/.opencodereview/rule.json` |
| 4 | System default | Embedded `system_rules.json` |

Rule format:

```json
{
  "rules": [
    { "path": "**/*.java", "rule": "Validate all required parameters for null" },
    { "path": "**/*mapper*.xml", "rule": "Check SQL for injection risks" }
  ]
}
```

---

## CI/CD Integration

```bash
ocr review \
  --from "origin/main" \
  --to "origin/feature-branch" \
  --format json
```

See [`examples/`](./examples/) for GitHub Actions and GitLab CI integration examples.

---

## Upstream Project

This fork tracks [alibaba/open-code-review](https://github.com/alibaba/open-code-review). For the original project documentation, design rationale, and community, refer to the upstream repository.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

## License

[Apache-2.0](LICENSE) — Upstream copyright 2026 Alibaba. Fork additions by contributors to this repository.
