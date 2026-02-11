# frank-blog-content-generator

CLI tool that generates blog content from R&D git commits using LLMs. Built for [Frankenstein AI Lab](https://github.com/frankenstein-ai) to turn daily research work into structured documentation without manual writing.

## What it generates

| Content type | Source | Description |
|---|---|---|
| **Notebooks** | git commits | Terse research summaries grouped by day or week |
| **Insight Memos** | git commits | Durable knowledge distilled from a body of work |
| **Blog Posts** | notebooks + memos | Long-form posts for a technical audience |
| **Homepage** | notebooks + memos | Up-to-date overview of latest research |

## Requirements

- Go 1.24+
- Git (available in `PATH`)
- An API key for OpenAI or Anthropic (not needed for Ollama)

## Install

**From GitHub Releases** (prebuilt binaries):

Download the latest binary for your platform from [Releases](https://github.com/frankenstein-ai/frank-blog-content-generator/releases).

**From source**:

```bash
go build -o frank .
```

## Quick start

```bash
# Build
go build -o frank .

# Generate notebooks (dry-run — no API key needed)
./frank generate notebooks \
  --source-repo /path/to/your-project \
  --output-dir ./output/notebooks \
  --dry-run

# Generate notebooks for real
export ANTHROPIC_API_KEY="sk-..."
./frank generate notebooks \
  --source-repo /path/to/your-project \
  --output-dir ./output/notebooks \
  --llm-provider anthropic

# Generate insight memos
./frank generate memos \
  --source-repo /path/to/your-project \
  --output-dir ./output/memos \
  --llm-provider anthropic

# Generate blog posts from existing notebooks and memos
./frank generate blog-posts \
  --notebooks-dir ./output/notebooks \
  --memos-dir ./output/memos \
  --output-dir ./output/posts \
  --llm-provider anthropic

# Generate homepage
./frank generate homepage \
  --notebooks-dir ./output/notebooks \
  --memos-dir ./output/memos \
  --output-file ./output/_index.md \
  --llm-provider anthropic

# Check processing state
./frank status
```

## Commands

```
frank generate notebooks   Generate research notebooks from git commits
frank generate memos       Generate insight memos from git commits
frank generate blog-posts  Generate blog posts from notebooks and memos
frank generate homepage    Generate homepage from notebooks and memos
frank status               Show last processed commit per source repo
frank --version            Print version
```

### Global flags

| Flag | Env var | Description |
|---|---|---|
| `--llm-provider` | `FRANK_LLM_PROVIDER` | LLM provider: `openai`, `anthropic`, or `ollama` |
| `--llm-model` | `FRANK_LLM_MODEL` | Model name (uses provider default if omitted) |
| `--state-db` | `FRANK_STATE_DB` | Path to SQLite state file (default: `.frank-state.db`) |
| `--dry-run` | — | Preview what would be generated without calling the LLM |

### Command-specific flags

| Flag | Env var | Used by |
|---|---|---|
| `--source-repo` | `FRANK_SOURCE_REPO` | `notebooks`, `memos` |
| `--output-dir` | `FRANK_OUTPUT_DIR` | `notebooks`, `memos`, `blog-posts` |
| `--notebooks-dir` | `FRANK_NOTEBOOKS_DIR` | `blog-posts`, `homepage` |
| `--memos-dir` | `FRANK_MEMOS_DIR` | `blog-posts`, `homepage` |
| `--output-file` | — | `homepage` |
| `--period` | — | `notebooks` (`day` or `week`) |

### API key env vars

| Provider | Env var |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| Ollama | `OLLAMA_HOST` (optional, default: `http://localhost:11434`) |

## How it works

```
Source repo (git commits)
        │
        ├──► Notebooks  (grouped by day/week)
        │
        └──► Insight Memos  (LLM identifies durable insights)
                    │
        ┌───────────┤
        │           │
        ▼           ▼
   Blog Posts    Homepage
```

1. **Read commits** — `frank` shells out to `git log` on the source repo
2. **Check state** — SQLite tracks the last processed commit per repo and content type, so only new commits are processed
3. **Group and prompt** — Commits are grouped (by time period for notebooks) and sent to the LLM with an embedded prompt template
4. **Parse and write** — LLM output is parsed into structured markdown files following opinionated naming conventions
5. **Update state** — The last processed commit is recorded so the next run picks up where this one left off

## Output file conventions

**Notebooks**: `{YYYY}-{MM}-{Topic-Slug}-{NN}.md`
- Topic slug is extracted from the LLM output (e.g., `LLM-Reasoning`, `ONNX-Export`)
- Example: `2025-02-LLM-Reasoning-01.md`

**Insight Memos**: `{YYYY}-{project}-insight-memo-{NNN}.md`
- Project name derived from source repo basename
- Example: `2025-mobile-agents-insight-memo-001.md`

**Blog Posts**: `{YYYY}-{MM}-{DD}-{slug}.md`
- Slug derived from the frontmatter title
- Hugo-compatible with `+++` frontmatter

**Homepage**: Single file at the path specified by `--output-file`

## Releases

Releases are automated via [GoReleaser](https://goreleaser.com). Pushing a version tag triggers the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This builds binaries for linux/darwin/windows (amd64 + arm64) and publishes them as a GitHub Release with checksums.

## GitHub Actions

The included workflow (`.github/workflows/generate.yaml`) runs daily at 06:00 UTC and can be triggered manually. It:

1. Checks out the source repo, lab-work repo, and blog repo
2. Builds the CLI
3. Generates all four content types
4. Commits and pushes results to the lab-work and blog repos

Required secrets: `GH_PAT`, `ANTHROPIC_API_KEY` (or `OPENAI_API_KEY`)

Optional variables: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`

## Tech stack

- **Go** with [Cobra](https://github.com/spf13/cobra) for CLI
- **LLM**: raw `net/http` calls to OpenAI, Anthropic, and Ollama APIs (no SDKs)
- **State**: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGo
- **Prompts**: embedded at compile time via `go:embed`

Only two external dependencies: `cobra` and `sqlite`.

## License

TBD
