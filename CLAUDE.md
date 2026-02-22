# CLAUDE.md — frank-blog-content-generator

## Project Overview

CLI tool that generates blog posts from a project's git history using LLMs.

- **Module**: `github.com/frankenstein-ai/frank-blog-content-generator`
- **Go**: 1.24, Cobra CLI framework, pure-Go SQLite (`modernc.org/sqlite`)
- **Binary name**: `frank`

## Build & Run

```bash
go build -o frank .          # build
go vet ./...                  # lint
```

Dry-run (no LLM API key needed):
```bash
./frank generate blog-posts --dry-run
```

No tests exist yet.

## Project Structure

```
cmd/
  root.go                    # root command
  status.go                  # status subcommand
  init.go                    # init subcommand (set starting commit + generate config)
  generate/
    generate.go              # generate parent command
    blogposts.go             # generate blog-posts
  update/
    update.go                # update parent command
    menu.go                  # update menu (hugo.toml)
    home.go                  # update home (regenerate homepage from published posts)
internal/
  config/                    # config resolution (flags > env vars > .frank.toml > defaults)
    config.go                # config struct and loaders
    toml.go                  # flat TOML parser for .frank.toml
  git/                       # git log reader (os/exec)
  hugo/                      # Hugo site operations (menu updates, post discovery)
  llm/                       # LLM provider interface + OpenAI/Anthropic implementations
  prompts/                   # embedded prompt templates (go:embed)
  generator/                 # blog post generator
  state/                     # SQLite state tracking
examples/
  workflow/                  # GitHub Actions workflow template
.github/workflows/
  generate.yaml              # GitHub Actions content generation workflow
  release.yaml               # GitHub Actions release workflow (GoReleaser)
```

## Architecture & Key Patterns

- **Source repo**: The CLI runs from the project directory — source repo is always `.` (current directory).
- **Config resolution**: CLI flags > env vars > `.frank.toml` > defaults. The `.frank.toml` file in the project root stores persistent config (flat key=value TOML, no nested tables). Env vars enable GitHub Actions compatibility. All `FRANK_`-prefixed except API keys (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`).
- **Hugo integration**: `internal/hugo/` handles menu updates, post discovery, and homepage regeneration. `frank update menu` finds the latest post and adds/replaces a "Latest:" entry in `hugo.toml`. `frank update home` reads published blog posts and regenerates the homepage via LLM.
- **LLM providers**: `Provider` interface in `internal/llm/llm.go`, implementations for OpenAI, Anthropic, Ollama, and OpenRouter using raw `net/http` (no SDKs). Factory via `llm.New(providerName)`. Ollama uses the OpenAI-compatible API with no auth header. OpenRouter uses the OpenAI-compatible API at `https://openrouter.ai/api/v1/chat/completions`.
- **Generator**: Blog posts are generated from git commits. Pipeline: read state → get commits → group by period (day/week) → for each group: fetch diffs → call LLM → write file → update state. The project README is included as context.
- **Prompts**: Embedded at compile time via `go:embed` in `internal/prompts/`. Single template in `blogposts.txt`.
- **State**: SQLite tracks last processed commit per (source_repo, content_type). DB file: `.frank-state.db`. The `init` command stores the parent of the specified commit so that the exclusive range naturally includes it. Content type: `blog-post`.
- **Dry-run mode**: Skips LLM provider creation entirely — no API keys required.

## Output File Naming

- **Blog posts**: `{YYYY}-{MM}-{DD}-{slug}.md` — slug from frontmatter title

## Code Conventions

- No external LLM SDKs — raw HTTP with retry (3 attempts, exponential backoff)
- No test framework yet
- Config env vars: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`, `FRANK_STATE_DB`, `FRANK_HUGO_DIR`
- API key env vars: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`
- Ollama env var: `OLLAMA_HOST` (default: `http://localhost:11434`)
