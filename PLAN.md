# frank-blog-content CLI — Implementation Plan

## Context

Frankenstein AI Lab does a lot of R&D but lacks time to write about it. This CLI tool automates content generation by reading git commits from R&D project repos and using LLMs to produce research notebooks, insight memos, blog posts, and a homepage. It's designed to run locally and in GitHub Actions.

## Key Decisions

| Decision | Choice |
|----------|--------|
| Language | Go |
| CLI framework | Cobra |
| LLM providers | OpenAI + Anthropic (configurable) |
| Git integration | `os/exec` (shell git commands) |
| LLM HTTP client | `net/http` (no SDK) |
| SQLite driver | `modernc.org/sqlite` (pure Go, no CGo) |
| Config | env vars + CLI flags (no config file) |
| Prompts | `embed.FS` (bundled in binary) |
| State tracking | Local SQLite file |

## Project Structure

```
frank-blog-content/
├── main.go
├── go.mod / go.sum
├── .gitignore
├── IDEA.md
├── PLAN.md
├── examples/
│   ├── notebook/                    # Reference notebook format
│   └── insight_memos/               # Reference memo format
├── cmd/
│   ├── root.go                      # Root cobra command, global flags
│   ├── status.go                    # `frank status` subcommand
│   └── generate/
│       ├── generate.go              # `generate` parent subcommand
│       ├── notebooks.go             # `generate notebooks`
│       ├── memos.go                 # `generate memos`
│       ├── blogposts.go             # `generate blog-posts`
│       └── homepage.go              # `generate homepage`
├── internal/
│   ├── config/config.go             # Env + flag resolution
│   ├── git/git.go                   # Read commits from external repo
│   ├── llm/
│   │   ├── llm.go                   # Provider interface + factory
│   │   ├── openai.go                # OpenAI implementation
│   │   └── anthropic.go             # Anthropic implementation
│   ├── state/state.go               # SQLite state tracking
│   ├── generator/
│   │   ├── notebooks.go
│   │   ├── memos.go
│   │   ├── blogposts.go
│   │   └── homepage.go
│   └── prompts/
│       ├── prompts.go               # Template loading (embed.FS)
│       ├── notebooks.txt
│       ├── memos.txt
│       ├── blogposts.txt
│       └── homepage.txt
└── .github/workflows/generate.yaml
```

## CLI Commands

```
frank generate notebooks  --source-repo <path> --output-dir <path> --period [day|week]
frank generate memos      --source-repo <path> --output-dir <path>
frank generate blog-posts  --notebooks-dir <path> --memos-dir <path> --output-dir <path>
frank generate homepage   --notebooks-dir <path> --memos-dir <path> --output-file <path>
frank status              # Show last processed commit per source repo
```

**Global flags**: `--llm-provider`, `--llm-model`, `--state-db`, `--dry-run`

**Env vars**: `FRANK_LLM_PROVIDER`, `FRANK_LLM_MODEL`, `FRANK_SOURCE_REPO`, `FRANK_OUTPUT_DIR`, `FRANK_STATE_DB`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`

## Output File Conventions

### Notebooks

- Filename: `{YYYY}-{MM}-{Topic-Slug}-{NN}.md`
- Example: `2025-02-LLM-Reasoning-01.md`
- The LLM provides a PascalCase topic slug (e.g., `LLM-Reasoning`, `ONNX-Export`), the system builds the full filename with year, month, and sequence number
- Heading matches filename: `# 2025-02-LLM-Reasoning-01`
- Sections: Question, Hypothesis, Setup, What I Tried, Results, Notes, Next
- Style: terse bullet points, bare metrics, no paragraphs

### Insight Memos

- Filename: `{YYYY}-{project}-insight-memo-{NNN}.md`
- Example: `2025-mobile-agents-insight-memo-001.md`
- Sequential numbering continues from existing files in the output directory
- Heading: `# Insight Memo: [Short Title]`
- Sections: Why This Matters, What We Found, When It Works, When It Fails, Recommendation
- Style: terse bullet points, one-line recommendation

### Blog Posts

- Filename: `{YYYY-MM-DD}-{slug-from-title}.md`
- Hugo frontmatter with `+++` delimiters
- 800-2000 words, technical audience

## Data Flow

```
Stage 1: git commits → Notebooks  (grouped by day/week)
Stage 2: git commits → Insight Memos  (LLM identifies insights)
Stage 3: Notebooks + Memos → Blog Posts  (reads markdown files from disk)
Stage 4: Notebooks + Memos → Homepage  (updates _index.md)
```

Stages 1-2 read from git independently. Stages 3-4 read from filesystem output of stages 1-2.

## SQLite Schema

```sql
CREATE TABLE processing_state (
    source_repo TEXT NOT NULL,
    content_type TEXT NOT NULL,  -- 'notebook', 'memo', 'blog-post', 'homepage'
    last_commit_hash TEXT NOT NULL,
    last_commit_timestamp TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (source_repo, content_type)
);

CREATE TABLE generated_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_repo TEXT NOT NULL,
    content_type TEXT NOT NULL,
    output_path TEXT NOT NULL,
    source_commits TEXT NOT NULL,  -- JSON array of commit hashes
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

## LLM Abstraction

```go
type Provider interface {
    Generate(ctx context.Context, req Request) (string, error)
}

type Request struct {
    SystemPrompt string
    UserPrompt   string
    MaxTokens    int
    Temperature  float64
}
```

Both providers use raw `net/http`. Retry with backoff on 429/5xx (3 retries max).

## Git Integration

Use `git -C <repo> log --format="%H%x00%s%x00%b%x00%an%x00%aI%x00" --name-status` with null-byte delimiters. Commits grouped by day or ISO week for notebooks. Use `git show --stat` for file-level summaries in LLM prompts.

## Dependencies (only 2 external)

```
github.com/spf13/cobra
modernc.org/sqlite
```

## Implementation Phases

### Phase 1: Skeleton [done]
- `go mod init`, `main.go`, Cobra root + generate subcommands
- `internal/config/config.go` with env var loading
- **Verify**: `go build && ./frank generate --help`

### Phase 2: Git + State [done]
- `internal/git/git.go` — parse git log output
- `internal/state/state.go` — SQLite open/read/write
- Wire into notebooks subcommand (print commits, no LLM)
- **Verify**: `./frank generate notebooks --source-repo <path>` prints commits

### Phase 3: LLM Integration [done]
- `internal/llm/` — Provider interface, OpenAI + Anthropic implementations
- `internal/prompts/` — embed prompt templates
- **Verify**: test with a real API call

### Phase 4: Generation Pipeline [done]
- `internal/generator/` — notebooks, memos, blogposts, homepage generators
- Wire generators into subcommands, write markdown output
- Opinionated file naming matching examples/ conventions
- **Verify**: end-to-end generation produces markdown files

### Phase 5: Polish + CI [done]
- `frank status` subcommand
- `--dry-run` support (skips LLM validation and API calls)
- `.github/workflows/generate.yaml`
- Error handling, logging

## Verification

1. `go build -o frank .` — compiles cleanly
2. `./frank generate notebooks --source-repo /path/to/any-git-repo --output-dir /tmp/test --dry-run` — shows what would be generated (no API key needed)
3. Run with a real API key against a real repo to produce markdown output
4. `./frank status` — shows last processed commits
