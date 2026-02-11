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
  generate/
    generate.go              # generate parent command
    notebooks.go             # generate notebooks
    memos.go                 # generate memos
    blogposts.go             # generate blog-posts
    homepage.go              # generate homepage
internal/
  config/                    # config resolution (flags > env vars > defaults)
  git/                       # git log reader (os/exec)
  llm/                       # LLM provider interface + OpenAI/Anthropic implementations
  prompts/                   # embedded prompt templates (go:embed)
  generator/                 # content generators (notebooks, memos, blogposts, homepage)
  state/                     # SQLite state tracking
examples/
  notebook/                  # reference notebook format
  insight_memos/             # reference memo format
.github/workflows/
  generate.yaml              # GitHub Actions workflow
```

## Architecture & Key Patterns

- **Config resolution**: CLI flags > env vars > defaults. Env vars enable GitHub Actions compatibility. All `FRANK_`-prefixed except API keys (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`).
- **LLM providers**: `Provider` interface in `internal/llm/llm.go`, implementations for OpenAI, Anthropic, and Ollama using raw `net/http` (no SDKs). Factory via `llm.New(providerName)`. Ollama uses the OpenAI-compatible API with no auth header.
- **Generators**: All follow the same pipeline — read state → get commits → group → call LLM → parse output → write files → update state.
- **Prompts**: Embedded at compile time via `go:embed` in `internal/prompts/`. Templates in `.txt` files.
- **State**: SQLite tracks last processed commit per (source_repo, content_type). DB file: `.frank-state.db`.
- **Dry-run mode**: Skips LLM provider creation entirely — no API keys required.

## Output File Naming Conventions

- **Notebooks**: `{YYYY}-{MM}-{Topic-Slug}-{NN}.md` — slug extracted from first line of LLM output
- **Memos**: `{YYYY}-{project}-insight-memo-{NNN}.md` — project derived from source repo basename
- **Blog posts**: `{YYYY}-{MM}-{DD}-{slug}.md` — slug from frontmatter title
- **Homepage**: single file specified via `--output-file`

## Code Conventions

- No external LLM SDKs — raw HTTP with retry (3 attempts, exponential backoff)
- No test framework yet
- Config env vars: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`, `FRANK_STATE_DB`, `FRANK_SOURCE_REPO`, `FRANK_OUTPUT_DIR`, `FRANK_NOTEBOOKS_DIR`, `FRANK_MEMOS_DIR`, `FRANK_BLOG_DIR`
- API key env vars: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`
- Ollama env var: `OLLAMA_HOST` (default: `http://localhost:11434`)
