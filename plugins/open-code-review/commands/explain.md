---
description: Explain a project, directory, or file with OpenCodeReview tutor mode.
---

Invoke OpenCodeReview (OCR) tutor mode to explain code without changing it.

## Workflow

### Step 1: Run Tutor Mode

Run OCR with agent-friendly output:

```bash
ocr explain --audience agent [user-args]
```

- Default: explains the current project up to the default file limit.
- If the user provides file or directory paths: pass them through as positional arguments.
- If the user asks for every file or connected architecture: pass `--all-files`.
- If the user wants durable output for a large repo: pass `--out explain-out`.
- If Graphify is missing for `--all-files`, run `ocr setup explain` or show the user `uv tool install graphifyy`.
- Never apply fixes, edit files, stage changes, or commit code in this command.

### Step 2: Report

Present the tutor output directly. Preserve:

- project overview
- file-by-file explanations
- connected architecture
- data/control flow
- dependencies and dependents
- gotchas or study notes

### Step 3: Follow-Up

If the user asks to go deeper, run targeted explanation:

```bash
ocr explain --audience agent path/to/file/or/dir
```

