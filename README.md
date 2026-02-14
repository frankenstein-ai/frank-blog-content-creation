# frank-blog-content-generator

CLI tool that generates blog content from R&D git commits using LLMs. Built for [Frankenstein AI Lab](https://github.com/frankenstein-ai) to turn daily research work into structured documentation without manual writing.

## What it generates

| Content type | Source | Description |
|---|---|---|
| **Notebooks** | git commits | Terse research summaries grouped by day or week, with code diff analysis |
| **Insight Memos** | git commits | Durable knowledge distilled from a work period, with code diff analysis |
| **Blog Posts** | notebooks + memos | Long-form posts for a technical audience |
| **Homepage** | notebooks + memos | Up-to-date overview of latest research |

## Requirements

- Go 1.24+
- Git (available in `PATH`)
- An API key for OpenAI, Anthropic, or OpenRouter (not needed for Ollama)

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

# Create a config file (optional — avoids repeating flags)
cat > .frank.toml <<'EOF'
hugo_dir = "/path/to/hugo-blog"
source_repo = "/path/to/your-project"
notebooks_dir = "./output/notebooks"
memos_dir = "./output/memos"
blog_dir = "./output/posts"
output_dir = "./output"
llm_provider = "anthropic"
EOF

# Set starting commits (skip old history)
./frank init \
  --source-repo /path/to/your-project \
  --commit abc1234 \
  --blog-repo /path/to/lab-work \
  --blog-commit def5678

# Generate notebooks + memos together (dry-run — no API key needed)
./frank generate notes \
  --notebooks-dir ./output/notebooks \
  --memos-dir ./output/memos \
  --dry-run

# Generate notebooks + memos for real (source repo read from state)
export ANTHROPIC_API_KEY="sk-..."
./frank generate notes \
  --notebooks-dir ./output/notebooks \
  --memos-dir ./output/memos \
  --llm-provider anthropic

# Or generate them individually:
./frank generate notebooks \
  --source-repo /path/to/your-project \
  --output-dir ./output/notebooks \
  --llm-provider anthropic

./frank generate memos \
  --source-repo /path/to/your-project \
  --output-dir ./output/memos \
  --llm-provider anthropic

# Generate blog posts from new notebooks and memos (source repo read from state)
./frank generate blog-posts \
  --notebooks-dir ./output/notebooks \
  --memos-dir ./output/memos \
  --output-dir ./output/posts \
  --llm-provider anthropic

# Generate homepage (output-file derived from hugo_dir if set)
./frank generate homepage \
  --notebooks-dir ./output/notebooks \
  --memos-dir ./output/memos \
  --llm-provider anthropic

# Update Hugo menu with latest blog post
./frank update menu

# Regenerate homepage from published blog posts
./frank update home

# Check processing state
./frank status
```

## Commands

```
frank generate notes       Generate notebooks and insight memos together
frank generate notebooks   Generate research notebooks from git commits
frank generate memos       Generate insight memos from git commits
frank generate blog-posts  Generate blog posts from notebooks and memos
frank generate homepage    Generate homepage from notebooks and memos
frank update menu          Update Hugo menu with the latest blog post
frank update home          Regenerate homepage from published blog posts
frank init                 Set starting commit point for content generation
frank status               Show last processed commit per source repo
frank --version            Print version
```

### Global flags

| Flag | Env var | Description |
|---|---|---|
| `--llm-provider` | `FRANK_LLM_PROVIDER` | LLM provider: `openai`, `anthropic`, `ollama`, or `openrouter` |
| `--llm-model` | `FRANK_LLM_MODEL` | Model name (uses provider default if omitted) |
| `--state-db` | `FRANK_STATE_DB` | Path to SQLite state file (default: `.frank-state.db`) |
| `--hugo-dir` | `FRANK_HUGO_DIR` | Path to Hugo site directory |
| `--dry-run` | — | Preview what would be generated without calling the LLM |

### Command-specific flags

| Flag | Env var | Used by |
|---|---|---|
| `--source-repo` | `FRANK_SOURCE_REPO` | `init`, `notes`, `notebooks`, `memos`, `blog-posts` |
| `--commit` | — | `init` (paired with `--source-repo`) |
| `--blog-repo` | `FRANK_BLOG_REPO` | `init` |
| `--blog-commit` | — | `init` (paired with `--blog-repo`) |
| `--output-dir` | `FRANK_OUTPUT_DIR` | `notebooks`, `memos`, `blog-posts` |
| `--notebooks-dir` | `FRANK_NOTEBOOKS_DIR` | `notes`, `blog-posts`, `homepage` |
| `--memos-dir` | `FRANK_MEMOS_DIR` | `notes`, `blog-posts`, `homepage` |
| `--output-file` | — | `homepage` (derived from `hugo_dir` when not set) |
| `--period` | — | `notes`, `notebooks`, `memos` (`day` or `week`) |

### API key env vars

| Provider | Env var |
|---|---|
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| Ollama | `OLLAMA_HOST` (optional, default: `http://localhost:11434`) |
| OpenRouter | `OPENROUTER_API_KEY` |

### Config file (`.frank.toml`)

Place a `.frank.toml` in the project root to avoid repeating flags. Flat key=value format:

```toml
# .frank.toml
hugo_dir = "/path/to/hugo-blog"
source_repo = "/path/to/your-project"
notebooks_dir = "./notebooks"
memos_dir = "./memos"
blog_dir = "./posts"
output_dir = "./output"
state_db = ".frank-state.db"
llm_provider = "anthropic"
llm_model = ""
```

Resolution order: **CLI flags > env vars > `.frank.toml` > defaults**

## How it works

```
Source repo (git commits)
        │
        ├──► Notebooks  (grouped by day/week, with code diffs)
        │
        └──► Insight Memos  (grouped by day/week, with code diffs)
                    │
        ┌───────────┤
        │           │
        ▼           ▼
   Blog Posts    Homepage
        │
        ├──► Hugo Menu  (frank update menu)
        │
        └──► Hugo Home  (frank update home)
```

1. **Read commits** — `frank` shells out to `git log` on the source repo
2. **Check state** — SQLite tracks the last processed commit per repo and content type, so only new commits are processed. Blog post generation also tracks commits in the blog content repo to discover only new notebooks and memos
3. **Group and fetch diffs** — Commits are grouped by time period, then each commit's full code diff and the project README are included as context for the LLM
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

Required secrets: `GH_PAT`, `ANTHROPIC_API_KEY` (or `OPENAI_API_KEY` or `OPENROUTER_API_KEY`)

Optional variables: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`

## Tech stack

- **Go** with [Cobra](https://github.com/spf13/cobra) for CLI
- **LLM**: raw `net/http` calls to OpenAI, Anthropic, Ollama, and OpenRouter APIs (no SDKs)
- **State**: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGo
- **Prompts**: embedded at compile time via `go:embed`

Only two external dependencies: `cobra` and `sqlite`.

## License

TBD
