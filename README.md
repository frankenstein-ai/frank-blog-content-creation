# frank-blog-content-generator

CLI tool that generates blog posts from your project's git history using LLMs. Built for [Frankenstein AI Lab](https://github.com/frankenstein-ai) to turn daily development work into published blog content without manual writing.

## How it works

```
Your project (git commits)
        │
        └──► Blog Posts  (grouped by day/week, with code diffs)
                 │
                 ├──► Hugo Menu  (frank update menu)
                 │
                 └──► Hugo Home  (frank update home)
```

1. **Read commits** — `frank` shells out to `git log` on the current project
2. **Check state** — SQLite tracks the last processed commit, so only new commits are processed
3. **Group and fetch diffs** — Commits are grouped by time period, then each commit's full code diff and the project README are included as context for the LLM
4. **Generate and write** — LLM output is parsed into Hugo-compatible markdown blog posts
5. **Update state** — The last processed commit is recorded so the next run picks up where this one left off

## Requirements

- Go 1.24+
- Git (available in `PATH`)
- An LLM provider: [GitHub Models](https://github.com/marketplace?type=models) (free), OpenAI, Anthropic, OpenRouter, or Ollama

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

# Initialize from your project directory (sets starting commit + generates config)
cd /path/to/your-project
./frank init --commit abc1234 --hugo-dir /path/to/hugo-blog

# Generate blog posts (dry-run — no API key needed)
./frank generate blog-posts --dry-run

# Generate blog posts for real
export ANTHROPIC_API_KEY="sk-..."
./frank generate blog-posts

# Update Hugo menu with latest blog post
./frank update menu

# Regenerate homepage from published blog posts
./frank update home

# Check processing state
./frank status
```

## Commands

```
frank init                              Initialize frank for this project (set starting commit + generate config)
frank generate blog-posts               Generate blog posts from git commits
frank update menu                       Update Hugo menu with the latest blog post
frank update home                       Regenerate homepage from published blog posts
frank update skill <name>               Download a skill definition from its upstream URL
frank status                            Show last processed commit
frank status update --commit <hash>     Move the last processed commit pointer
frank --version                         Print version
```

### Global flags

| Flag | Env var | Description |
|---|---|---|
| `--llm-provider` | `FRANK_LLM_PROVIDER` | LLM provider: `github`, `openai`, `anthropic`, `ollama`, or `openrouter` |
| `--llm-model` | `FRANK_LLM_MODEL` | Model name (uses provider default if omitted) |
| `--state-db` | `FRANK_STATE_DB` | Path to SQLite state file (default: `.frank-state.db`) |
| `--hugo-dir` | `FRANK_HUGO_DIR` | Path to Hugo site directory |
| `--dry-run` | — | Preview what would be generated without calling the LLM |

### Command-specific flags

| Flag | Env var | Used by |
|---|---|---|
| `--commit` | — | `init` (required — starting commit hash), `status update` (required — target commit hash) |
| `--period` | — | `blog-posts` (`day` or `week`, default: `week`) |

### API key env vars

| Provider | Env var |
|---|---|
| GitHub Models | `GITHUB_TOKEN` (PAT with `models:read` scope) |
| OpenAI | `OPENAI_API_KEY` |
| Anthropic | `ANTHROPIC_API_KEY` |
| Ollama | `OLLAMA_HOST` (optional, default: `http://localhost:11434`) |
| OpenRouter | `OPENROUTER_API_KEY` |

### Config file (`.frank.toml`)

Place a `.frank.toml` in the project root to avoid repeating flags. Created automatically by `frank init`. Flat key=value format:

```toml
# .frank.toml
hugo_dir = "/path/to/hugo-blog"
state_db = ".frank-state.db"
llm_provider = "anthropic"
llm_model = ""
# day or week
period = "week"
# post-processing skills
# skills = ["humanizer"]
# upstream URLs for skills (used by `frank update skill <name>`)
# skill_url_humanizer = "https://raw.githubusercontent.com/blader/humanizer/main/SKILL.md"
```

Resolution order: **CLI flags > env vars > `.frank.toml` > defaults**

## Output file conventions

**Blog Posts**: `{YYYY}-{MM}-{DD}-{slug}.md`
- Slug derived from the frontmatter title
- Hugo-compatible with `+++` TOML frontmatter

## Releases

Releases are automated via [GoReleaser](https://goreleaser.com). Pushing a version tag triggers the release workflow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This builds binaries for linux/darwin/windows (amd64 + arm64) and publishes them as a GitHub Release with checksums.

## GitHub Actions

### Reusable workflow (recommended)

A reusable workflow is provided at `.github/workflows/generate-reusable.yaml`. It's fully self-contained — no local setup, no `frank init`, no config files needed. The default LLM provider is GitHub Models (free tier).

Add this to `.github/workflows/generate.yaml` in your project:

```yaml
name: Generate Blog Posts

on:
  push:
    branches: [main]
  workflow_dispatch:

permissions:
  contents: write

jobs:
  blog:
    uses: frankenstein-ai/frank-blog-content-generator/.github/workflows/generate-reusable.yaml@main
    with:
      blog-repo: your-org/your-blog
      skill-urls: |
        humanizer=https://raw.githubusercontent.com/blader/humanizer/main/SKILL.md
    secrets:
      gh-pat: ${{ secrets.GH_PAT }}
```

**Setup:**

1. Create a `GH_PAT` secret — a GitHub PAT with `repo` + `models:read` scope
2. Set `blog-repo` to your Hugo blog repository
3. That's it — no local setup needed

The workflow triggers on every push to main. On first run it sets a baseline at HEAD (no posts generated). Subsequent pushes generate blog posts from new commits. The state DB (`.frank-state.db`) is auto-committed back to your repo with `[skip ci]` to prevent loops. Skills are downloaded, posts are generated, and everything is pushed to the blog repo.

**Optional inputs:** `start-commit` (process older commits on first run), `period` (day/week), `frank-version`, `llm-provider`, `llm-model`, `commit-message`, `skill-urls`. To use a paid provider, set `llm-provider` and pass the corresponding secret (`anthropic-api-key`, `openai-api-key`, or `openrouter-api-key`).

See the full input/secret reference in [generate-reusable.yaml](.github/workflows/generate-reusable.yaml) and a complete example in [examples/workflow/generate-blog-posts.yaml](examples/workflow/generate-blog-posts.yaml).

### Development workflow

The included `.github/workflows/generate.yaml` is used by this repository itself — it builds frank from source and generates blog posts daily. For other projects, use the reusable workflow above.

## Tech stack

- **Go** with [Cobra](https://github.com/spf13/cobra) for CLI
- **LLM**: raw `net/http` calls to GitHub Models, OpenAI, Anthropic, Ollama, and OpenRouter APIs (no SDKs)
- **State**: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGo
- **Prompts**: embedded at compile time via `go:embed`

Only two external dependencies: `cobra` and `sqlite`.

## License

TBD
