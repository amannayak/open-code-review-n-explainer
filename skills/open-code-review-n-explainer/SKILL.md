---
name: open-code-review-n-explainer
description: >
  Performs AI-powered code review and read-only code explanation using the `ocr` CLI from
  amannayak/open-code-review-n-explainer. Use when the user asks to review code, review
  a pull request, review staged/unstaged changes, review a commit, or
  compare branches for code quality issues. Also use when the user asks to
  explain a project, directory, file, symbol, or code flow. Produces line-level review
  comments and can automatically apply fixes when requested. With appropriate
  review rules, can detect various types of issues including bugs, security
  vulnerabilities, performance problems, and code quality concerns.
license: Apache-2.0
compatibility: >
  Requires the `ocr` CLI installed (build from source or GitHub release binary).
  Requires a configured LLM (Anthropic or OpenAI-compatible) before first run.
metadata:
  author: amannayak
  homepage: https://github.com/amannayak/open-code-review-n-explainer
  version: "1.0.0"
---

# Open Code Review

A skill for invoking [open-code-review-n-explainer](https://github.com/amannayak/open-code-review-n-explainer) (`ocr`) — a fork of alibaba/open-code-review that adds read-only tutor mode via `ocr explain`. Generates structured, line-level review comments and can deeply explain code architecture.

## Prerequisites check

Before starting a review, verify the environment:

```bash
# 1. Check the CLI is installed
which ocr || echo "NOT INSTALLED"

# 2. Verify LLM connectivity
ocr llm test
```

If `ocr` is not installed, build from source:

```bash
git clone https://github.com/amannayak/open-code-review-n-explainer.git
cd open-code-review-n-explainer && make build
sudo cp dist/opencodereview /usr/local/bin/ocr
```

If `ocr llm test` fails, the user must configure an LLM. Guide them with one of these options:

**Option A — Environment variables (highest priority, recommended for CI):**

```bash
export OCR_LLM_URL=https://api.anthropic.com/v1/messages
export OCR_LLM_TOKEN=<api-key>
export OCR_LLM_MODEL=claude-opus-4-6
export OCR_USE_ANTHROPIC=true
```

**Option B — Persistent config:**

```bash
ocr config set llm.url https://api.anthropic.com/v1/messages
ocr config set llm.auth_token <api-key>
ocr config set llm.model claude-opus-4-6
ocr config set llm.use_anthropic true
```

**Local OpenAI-compatible models:**

```bash
ocr config set llm.url http://localhost:11434/v1/chat/completions
ocr config set llm.model gemma-local
ocr config set llm.use_anthropic false
ocr config set llm.allow_local_no_token true
```

Stop here and ask the user to provide credentials — never invent or hardcode API keys.

## Review Workflow

### Step 1: Gather Business Context

Analyze the review target (commits, branch, or changes) to extract concise business context. Pass this context via `--background` to improve review quality.

### Step 2: Run Code Review

Run the OCR command with appropriate flags. **Always pass business context via `--background`** when available:

```bash
ocr review --audience agent --background "business context here" [user-args]
```

**Argument handling:**

- **Background context** (RECOMMENDED): use `--background "context"` or `-b "context"` to provide business context for better review quality
- **Default** (no user arguments): reviews staged, unstaged, and untracked changes (workspace mode)
- **Specific commit**: use `--commit` or `-c` to review a single commit against its parent
- **Branch comparison**: use `--from <ref>` and `--to <ref>` to review diff between two refs
- **Timeout**: default timeout is 10 minutes per file; adjust with `--timeout <minutes>`
- **Concurrency**: default concurrency is 8 file workers; reduce with `--concurrency <n>` if rate limits are hit
- **Preview mode**: use `--preview` or `-p` to preview which files will be reviewed without running the LLM
- **Installation**: if `ocr` command is not found, build from source: `git clone https://github.com/amannayak/open-code-review-n-explainer.git && cd open-code-review-n-explainer && make build && sudo cp dist/opencodereview /usr/local/bin/ocr`

**Common invocation patterns:**

| User says | Command to run |
|-----------|---------------|
| "review my changes" / "review the working copy" | `ocr review --audience agent -b "context"` |
| "review this PR" / "review feature branch" | `ocr review --audience agent -b "context" --from main --to <branch>` |
| "review commit abc123" | `ocr review --audience agent -b "context" --commit abc123` |
| "what would be reviewed?" (dry-run) | `ocr review --preview` |

**Output mode:**

- Always use `--audience agent` to suppress progress UI and emit only the final summary

### Step 3: Classify and Report

For each comment from the review output, classify by priority and report all issues to the user:

- **High**: Obvious bugs, security issues, clear mistakes, or well-founded suggestions with precise fix proposals
- **Medium**: Reasonable concerns but context-dependent, style/performance suggestions, or fixes that require manual implementation
- **Low**: Likely false positives, lacking sufficient context, nitpicks, or meaningless suggestions

Report all comments grouped by priority level.

### Step 4: Fix

Before applying fixes, check whether the user requested automatic fixes:

- If the user explicitly requested "review and fix" or similar, proceed with automatic fixes
- If the user only requested "review" without fix intent, ask for permission before applying any changes

When fixing issues and suggestions:

- Focus on High and Medium priority items
- Apply fixes directly to the code when safe and well-defined
- For complex fixes requiring manual intervention, clearly describe what needs to be done
- Always verify fixes with the user before committing

## Output Format

Each comment contains:

- `path`: File path
- `content`: Review comment text
- `start_line` / `end_line`: Line range (both 0 means positioning failed)
- `suggestion_code`: Optional fix suggestion
- `existing_code`: Optional original code snippet
- `thinking`: Optional LLM reasoning process

After filtering comments by priority, present results using this template:

```markdown
## Code Review Results

**Files reviewed**: N
**Issues found**: X high priority / Y medium priority

### High Priority

- **`path/to/file.java:42`** — Brief description
  > Recommendation: How to fix

### Medium Priority

- **`path/to/file.ts:88`** — Brief description
  > Recommendation: How to fix (if applicable)
```

If the review found no issues after filtering, simply state: "Review complete — no issues found in N files."

**Priority classification:**

- **High**: Obvious bugs, security issues, clear mistakes, or well-founded suggestions with precise fix proposals
- **Medium**: Reasonable concerns but context-dependent, style/performance suggestions, or fixes that require manual implementation
- **Low**: Discarded silently (likely false positives, lacking context, nitpicks, or meaningless suggestions)

**Handling mispositioned comments:**

When `start_line` and `end_line` are both `0`, the comment failed to locate the exact position in the file. In such cases:

1. Read the comment content to understand the issue
2. Examine the target file mentioned in the comment
3. Identify the relevant code section based on the comment's context
4. Apply the fix or suggestion to the correct location

## Custom Review Rules

If the user wants project-specific rules, OCR resolves them in this priority order:

1. `--rule <path>` flag (highest)
2. `<repo>/.opencodereview/rule.json`
3. `~/.opencodereview/rule.json`
4. Built-in system defaults (lowest)

Rule file format:

```json
{
  "rules": [
    {
      "path": "**/*.java",
      "rule": "All new methods must validate required parameters for null"
    },
    {
      "path": "**/*mapper*.xml",
      "rule": "Check SQL for injection risks and missing closing tags"
    }
  ]
}
```

To preview which rule applies to a file before reviewing:

```bash
ocr rules check src/main/java/com/example/Foo.java
```

## Gotchas

- **LLM must be configured first** — `ocr review` will fail loudly if no LLM is reachable. Always run `ocr llm test` before the first review.
- **Working directory matters** — `ocr review` operates on the Git repo at the current directory. Use `--repo /path/to/repo` to run from elsewhere.
- **Untracked files are reviewed in workspace mode** — running bare `ocr review` includes staged, unstaged, *and* untracked changes. Stage selectively if you want narrower scope.
- **Large diffs may hit token limits** — files with very large diffs may be truncated. The default `MAX_TOKENS` is 58888 per request.
- **Plan phase triggers at 50 lines** — diffs exceeding 50 changed lines run an extra risk-analysis phase before main review. This adds latency but improves quality.
- **Don't pass `--audience human`** — it streams progress UI that pollutes output. Always use `--audience agent`.
- **Comment language follows config** — set `language` config to `English` or `Chinese` (default: Chinese) to control review comment language.

## Validation

After the review completes, verify success by checking:

1. The command exited with code 0
2. Comments were generated (or "No comments generated" message appears)
3. Warnings (if any) are displayed in stderr

If errors occurred, check the stderr warnings for details about which files failed and why.

## Explain Workflow

Use tutor mode when the user asks to understand code rather than critique or fix it.

### Step 1: Run Explanation

```bash
ocr explain --audience agent [user-args]
```

Argument handling:

- Specific file or directory: pass paths through, e.g. `ocr explain --audience agent internal/agent`
- Whole project, default limit: `ocr explain --audience agent`
- Every file plus connected architecture: `ocr explain --audience agent --all-files`
- Large repo artifacts: `ocr explain --audience agent --all-files --out explain-out`
- Existing Graphify graph: `ocr explain --audience agent --all-files --graph graphify-out/graph.json`

For `--all-files`, Graphify is required. If missing, run:

```bash
ocr setup explain
```

or tell the user to run:

```bash
uv tool install graphifyy
```

### Step 2: Report

Preserve the tutor structure:

- project overview
- file-by-file explanations
- connected architecture
- runtime/data flow
- dependencies and dependents
- gotchas and study notes

### Step 3: Stay Read-Only

Do not edit files, apply fixes, stage changes, or commit code as part of tutor mode. If the user asks to change code after an explanation, treat that as a separate implementation request.

## References

- Full docs: https://github.com/amannayak/open-code-review-n-explainer
- Upstream project: https://github.com/alibaba/open-code-review
- Issue tracker: https://github.com/amannayak/open-code-review-n-explainer/issues
