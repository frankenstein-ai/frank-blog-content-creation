# CLAUDE.md — frank-blog-content-generator

## Project Overview

CLI tool that generates blog content (notebooks, insight memos, blog posts, homepage updates) from R&D git commits using LLMs.

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
./frank generate notebooks --source-repo /path/to/repo --output-dir ./out --dry-run
```

No tests exist yet.

## Project Structure

```
cmd/
  root.go                    # root command
  status.go                  # status subcommand
  init.go                    # init subcommand (set starting commit)
  generate/
    generate.go              # generate parent command
    notes.go                 # generate notebooks + memos together
    notebooks.go             # generate notebooks
    memos.go                 # generate memos
    blogposts.go             # generate blog-posts
    homepage.go              # generate homepage
  update/
    update.go                # update parent command
    menu.go                  # update menu (hugo.toml)
internal/
  config/                    # config resolution (flags > env vars > .frank.toml > defaults)
    config.go                # config struct and loaders
    toml.go                  # flat TOML parser for .frank.toml
  git/                       # git log reader (os/exec)
  hugo/                      # Hugo site operations (menu updates, post discovery)
  llm/                       # LLM provider interface + OpenAI/Anthropic implementations
  prompts/                   # embedded prompt templates (go:embed)
  generator/                 # content generators (notebooks, memos, blogposts, homepage)
  state/                     # SQLite state tracking
examples/
  notebook/                  # reference notebook format
  insight_memos/             # reference memo format
.github/workflows/
  generate.yaml              # GitHub Actions content generation workflow
  release.yaml               # GitHub Actions release workflow (GoReleaser)
```

## Architecture & Key Patterns

- **Config resolution**: CLI flags > env vars > `.frank.toml` > defaults. The `.frank.toml` file in the project root stores persistent paths (flat key=value TOML, no nested tables, zero new deps). Env vars enable GitHub Actions compatibility. All `FRANK_`-prefixed except API keys (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`).
- **Hugo integration**: `internal/hugo/` handles menu updates and post discovery. `frank update menu` finds the latest post and adds/replaces a "Latest:" entry in `hugo.toml`.
- **LLM providers**: `Provider` interface in `internal/llm/llm.go`, implementations for OpenAI, Anthropic, Ollama, and OpenRouter using raw `net/http` (no SDKs). Factory via `llm.New(providerName)`. Ollama uses the OpenAI-compatible API with no auth header. OpenRouter uses the OpenAI-compatible API at `https://openrouter.ai/api/v1/chat/completions`.
- **Generators**: Notebooks and memos group commits by time period (day/week), fetch code diffs for each commit within the group, and include the project README as context. Pipeline: read state → get commits → group by period → for each group: fetch diffs → call LLM → write file → update state.
- **Prompts**: Embedded at compile time via `go:embed` in `internal/prompts/`. Templates in `.txt` files.
- **State**: SQLite tracks last processed commit per (source_repo, content_type). DB file: `.frank-state.db`. The `init` command stores the parent of the specified commit so that the exclusive range naturally includes it.
- **Dry-run mode**: Skips LLM provider creation entirely — no API keys required.

## Output File Naming Conventions

- **Notebooks**: `{YYYY}-{MM}-{Topic-Slug}-{NN}.md` — slug extracted from first line of LLM output
- **Memos**: `{YYYY}-{project}-insight-memo-{NNN}.md` — project derived from source repo basename
- **Blog posts**: `{YYYY}-{MM}-{DD}-{slug}.md` — slug from frontmatter title
- **Homepage**: single file specified via `--output-file` (derived from `hugo_dir` when not set)

## Code Conventions

- No external LLM SDKs — raw HTTP with retry (3 attempts, exponential backoff)
- No test framework yet
- Config env vars: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`, `FRANK_STATE_DB`, `FRANK_SOURCE_REPO`, `FRANK_BLOG_REPO`, `FRANK_OUTPUT_DIR`, `FRANK_NOTEBOOKS_DIR`, `FRANK_MEMOS_DIR`, `FRANK_BLOG_DIR`, `FRANK_HUGO_DIR`
- API key env vars: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`
- Ollama env var: `OLLAMA_HOST` (default: `http://localhost:11434`)
