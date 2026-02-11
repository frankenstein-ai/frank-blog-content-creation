# Blog Content Generation from R&D

We're Frankenstein AI Lab, a startup focused on AI R&D. We do a lot of research but don't have time to write about it properly. This CLI tool automatically generates content from our git commits.

## What It Does

Reads git commits from R&D project repos and uses LLMs to generate:

1. **Notebooks** — research summaries for a time period (day/week), terse and structured
2. **Insight Memos** — durable knowledge learned about a topic
3. **Blog Posts** — markdown posts generated from notebooks and memos
4. **Homepage** — up-to-date overview of latest research and WIP

## How It Works

- Reads commits from a **separate source repo** (our R&D projects)
- Tracks the last processed commit in a **local SQLite file** so it only processes new work
- Uses **LLMs** (OpenAI or Anthropic, configurable) to generate structured markdown
- Outputs follow opinionated file naming and content conventions (see examples/)

## Output Conventions

### Notebooks

- Filename: `{YYYY}-{MM}-{Topic-Slug}-{NN}.md` (e.g., `2025-02-LLM-Reasoning-01.md`)
- Heading matches filename: `# 2025-02-LLM-Reasoning-01`
- Sections: Question, Hypothesis, Setup, What I Tried, Results, Notes, Next
- Style: terse bullet points, bare metrics, no filler

### Insight Memos

- Filename: `{YYYY}-{project}-insight-memo-{NNN}.md` (e.g., `2025-mobile-agents-insight-memo-001.md`)
- Heading: `# Insight Memo: [Short Title]`
- Sections: Why This Matters, What We Found, When It Works, When It Fails, Recommendation
- Style: terse bullet points, one-line recommendation

## UX

Configure via CLI flags and/or env vars:
- Source project repo path
- Output directories for notebooks, memos, blog posts
- LLM provider and model
- Supports `--dry-run` to preview without generating

## Tech Stack

- **Go** CLI using Cobra
- **LLM**: OpenAI + Anthropic (configurable, raw net/http — no SDKs)
- **State**: SQLite via `modernc.org/sqlite` (pure Go, no CGo)
- **Prompts**: embedded templates (`embed.FS`)
- **CI**: GitHub Actions workflow for scheduled/manual generation
