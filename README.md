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
| `--llm-provider` | `FRANK_LLM_PROVIDER` | LLM provider: `openai`, `anthropic`, `ollama`, or `openrouter` |
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

The included workflow (`.github/workflows/generate.yaml`) runs daily at 06:00 UTC and can be triggered manually. It:

1. Checks out the source repo and blog repo
2. Builds the CLI
3. Generates blog posts from new commits
4. Updates the Hugo menu and homepage
5. Commits and pushes results to the blog repo

Required secrets: `GH_PAT`, `ANTHROPIC_API_KEY` (or `OPENAI_API_KEY` or `OPENROUTER_API_KEY`)

Optional variables: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`

### Workflow template

A reusable workflow template is provided in `examples/workflow/generate-blog-posts.yaml`. Drop it into any project's `.github/workflows/` to auto-generate blog posts from commits.

**Setup:**

1. Copy the template to `.github/workflows/` in your project
2. Set secrets: `GH_PAT` + an LLM API key (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `OPENROUTER_API_KEY`)
3. Edit the `env` block at the top of the workflow to set your blog repo
4. (Optional) Set repo variables: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`
5. (Optional) Pin `FRANK_VERSION` to a specific release tag for reproducibility

## Tech stack

- **Go** with [Cobra](https://github.com/spf13/cobra) for CLI
- **LLM**: raw `net/http` calls to OpenAI, Anthropic, Ollama, and OpenRouter APIs (no SDKs)
- **State**: [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) — pure Go, no CGo
- **Prompts**: embedded at compile time via `go:embed`

Only two external dependencies: `cobra` and `sqlite`.

## License

TBD
