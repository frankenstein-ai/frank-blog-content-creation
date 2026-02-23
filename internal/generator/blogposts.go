package generator

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/git"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/hugo"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/llm"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/prompts"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/skills"
	"github.com/frankenstein-ai/frank-blog-content-generator/internal/state"
)

type topicGroup struct {
	Topic   string
	Commits []string
}

type GenerateResult struct {
	OutputPath string
	Content    string
	Commits    []string
}

type BlogPostGenerator struct {
	LLM           llm.Provider
	State         *state.Store
	Templates     *prompts.Templates
	Skills        []skills.Skill
	SourceRepo    string
	OutputDir     string
	Period        string // "day" or "week"
	ReadmeContent string
	DryRun        bool
}

func (g *BlogPostGenerator) Generate(ctx context.Context) ([]GenerateResult, error) {
	repoName := git.RepoName(g.SourceRepo)

	lastHash, err := g.State.GetLastCommit(g.SourceRepo, "blog-post")
	if err != nil {
		return nil, fmt.Errorf("getting last commit: %w", err)
	}

	commits, err := git.GetCommits(g.SourceRepo, lastHash)
	if err != nil {
		return nil, fmt.Errorf("reading commits: %w", err)
	}

	if len(commits) == 0 {
		fmt.Println("No new commits to process for blog posts.")
		return nil, nil
	}

	fmt.Printf("Found %d new commits for blog posts\n", len(commits))

	var groups map[string][]git.Commit
	switch g.Period {
	case "day":
		groups = git.GroupByDay(commits)
	default:
		groups = git.GroupByWeek(commits)
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if err := os.MkdirAll(g.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	var results []GenerateResult
	for _, key := range keys {
		groupCommits := groups[key]

		// Fetch diffs for all commits in this group
		diffs := make(map[string]string)
		for _, c := range groupCommits {
			diff, err := git.GetCommitDiff(g.SourceRepo, c.Hash)
			if err != nil {
				shortHash := c.Hash
				if len(shortHash) > 8 {
					shortHash = shortHash[:8]
				}
				return nil, fmt.Errorf("getting diff for %s: %w", shortHash, err)
			}
			diffs[c.Hash] = diff
		}

		// Determine sub-groups: split by topic if many commits, otherwise single group
		type subGroup struct {
			topic   string
			commits []git.Commit
		}
		var subGroups []subGroup

		if len(groupCommits) >= 10 && !g.DryRun {
			topics := g.planTopics(ctx, groupCommits)

			// Build a lookup from short hash to commit
			commitByShortHash := make(map[string]git.Commit)
			for _, c := range groupCommits {
				short := c.Hash
				if len(short) > 8 {
					short = short[:8]
				}
				commitByShortHash[short] = c
			}

			for _, tg := range topics {
				var matched []git.Commit
				for _, h := range tg.Commits {
					if c, ok := commitByShortHash[h]; ok {
						matched = append(matched, c)
					}
				}
				if len(matched) > 0 {
					subGroups = append(subGroups, subGroup{topic: tg.Topic, commits: matched})
				}
			}

			// Fallback if all commits were unmatched
			if len(subGroups) == 0 {
				subGroups = []subGroup{{topic: "", commits: groupCommits}}
			}
		} else {
			subGroups = []subGroup{{topic: "", commits: groupCommits}}
		}

		if g.DryRun {
			if len(groupCommits) >= 10 {
				fmt.Printf("[dry-run] Would generate multiple blog posts for %s (%d commits, topic splitting enabled)\n", key, len(groupCommits))
			} else {
				fmt.Printf("[dry-run] Would generate blog post for %s (%d commits)\n", key, len(groupCommits))
			}
			continue
		}

		for _, sg := range subGroups {
			topicLabel := key
			if sg.topic != "" {
				topicLabel = fmt.Sprintf("%s — %s", key, sg.topic)
			}

			// Date from first commit in sub-group for accurate filenames
			date := sg.commits[0].Timestamp.Format("2006-01-02T15:04:05-07:00")

			userPrompt := buildUserPrompt(repoName, key, sg.commits, diffs, g.ReadmeContent)

			fmt.Printf("Generating blog post for %s (%d commits)...\n", topicLabel, len(sg.commits))

			systemPrompt := strings.Replace(g.Templates.BlogPost, "{{.Date}}", date, 1)

			content, err := g.LLM.Generate(ctx, llm.Request{
				SystemPrompt: systemPrompt,
				UserPrompt:   userPrompt,
				MaxTokens:    16384,
				Temperature:  0.7,
			})
			if err != nil {
				return nil, fmt.Errorf("generating blog post for %s: %w", topicLabel, err)
			}

			// Run post-processing skills
			for _, skill := range g.Skills {
				fmt.Printf("  Running skill '%s' on blog post for %s...\n", skill.Name, topicLabel)
				frontmatter, body := hugo.SplitFrontmatter(content)
				skillInput := "Rewrite the following blog post body applying all patterns from your instructions. Output ONLY the rewritten markdown body — no preamble, no commentary, no notes.\n\n" + body
				processed, err := g.LLM.Generate(ctx, llm.Request{
					SystemPrompt: skill.Prompt,
					UserPrompt:   skillInput,
					MaxTokens:    16384,
					Temperature:  0.4,
				})
				if err != nil {
					return nil, fmt.Errorf("skill '%s' for %s: %w", skill.Name, topicLabel, err)
				}
				processed = hugo.SanitizeLLMOutput(processed)
				processed = stripMetaCommentary(processed)
				content = frontmatter + "\n" + strings.TrimSpace(processed) + "\n"
			}

			slug := extractSlug(content)
			datePrefix := sg.commits[0].Timestamp.Format("2006-01-02")
			filename := fmt.Sprintf("%s-%s.md", datePrefix, slug)
			outPath := filepath.Join(g.OutputDir, filename)

			if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
				return nil, fmt.Errorf("writing blog post: %w", err)
			}

			hashes := make([]string, len(sg.commits))
			for j, c := range sg.commits {
				hashes[j] = c.Hash
			}

			if err := g.State.RecordGeneration(g.SourceRepo, "blog-post", outPath, hashes); err != nil {
				return nil, fmt.Errorf("recording generation: %w", err)
			}

			results = append(results, GenerateResult{
				OutputPath: outPath,
				Content:    content,
				Commits:    hashes,
			})

			fmt.Printf("  Written: %s\n", outPath)
		}
	}

	// Update last processed commit (use the most recent commit)
	if len(commits) > 0 && !g.DryRun {
		newest := commits[0] // git log returns newest first
		if err := g.State.SetLastCommit(g.SourceRepo, "blog-post", newest.Hash, newest.Timestamp); err != nil {
			return nil, fmt.Errorf("updating state: %w", err)
		}
	}

	return results, nil
}

func buildUserPrompt(repoName, period string, commits []git.Commit, diffs map[string]string, readmeContent string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n", repoName)
	fmt.Fprintf(&b, "Period: %s\n", period)

	if readmeContent != "" {
		fmt.Fprintf(&b, "\nProject description (from README):\n%s\n", readmeContent)
	}

	fmt.Fprintf(&b, "\nCommits (%d total):\n\n", len(commits))

	maxDiffPerCommit := 15000 / len(commits)
	if maxDiffPerCommit < 2000 {
		maxDiffPerCommit = 2000
	}

	for _, c := range commits {
		shortHash := c.Hash
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}

		fmt.Fprintf(&b, "---\n")
		fmt.Fprintf(&b, "Hash: %s\n", shortHash)
		fmt.Fprintf(&b, "Date: %s\n", c.Timestamp.Format("2006-01-02 15:04"))
		fmt.Fprintf(&b, "Author: %s\n", c.Author)
		fmt.Fprintf(&b, "Subject: %s\n", c.Subject)
		if c.Body != "" {
			fmt.Fprintf(&b, "Description:\n%s\n", c.Body)
		}
		if len(c.Files) > 0 {
			fmt.Fprintf(&b, "Files changed:\n")
			for _, f := range c.Files {
				fmt.Fprintf(&b, "  %s %s\n", f.Status, f.Path)
			}
		}

		if diff, ok := diffs[c.Hash]; ok && diff != "" {
			truncatedDiff := diff
			if len(truncatedDiff) > maxDiffPerCommit {
				truncatedDiff = truncatedDiff[:maxDiffPerCommit] + "\n... (diff truncated)"
			}
			fmt.Fprintf(&b, "Code changes (diff):\n```\n%s\n```\n", truncatedDiff)
		}
		fmt.Fprintf(&b, "---\n\n")
	}

	b.WriteString("Write a blog post covering this work. Focus on what makes it interesting and useful for other developers.")
	return b.String()
}

// dominantDirAtDepth returns the directory prefix (up to `depth` components)
// where a commit changed the most files.
func dominantDirAtDepth(files []git.FileChange, depth int) string {
	if len(files) == 0 {
		return "other"
	}
	counts := make(map[string]int)
	for _, f := range files {
		parts := strings.Split(f.Path, "/")
		// Remove the filename (last component)
		if len(parts) > 1 {
			dirParts := parts[:len(parts)-1]
			if len(dirParts) > depth {
				dirParts = dirParts[:depth]
			}
			counts[strings.Join(dirParts, "/")]++
		} else {
			counts["root"]++
		}
	}

	var maxDir string
	var maxCount int
	for dir, count := range counts {
		if count > maxCount {
			maxDir = dir
			maxCount = count
		}
	}
	return maxDir
}

// sharedPrefixLen returns the number of shared path components between two paths.
func sharedPrefixLen(a, b string) int {
	aParts := strings.Split(a, "/")
	bParts := strings.Split(b, "/")
	n := len(aParts)
	if len(bParts) < n {
		n = len(bParts)
	}
	shared := 0
	for i := 0; i < n; i++ {
		if aParts[i] != bParts[i] {
			break
		}
		shared++
	}
	return shared
}

// groupByFilePath groups commits by their dominant file path prefix.
// Always produces a valid result with no LLM calls.
func groupByFilePath(commits []git.Commit) []topicGroup {
	// Step 1: Group commits by dominant directory (2-level depth)
	dirCommits := make(map[string][]git.Commit)
	for _, c := range commits {
		dir := dominantDirAtDepth(c.Files, 2)
		dirCommits[dir] = append(dirCommits[dir], c)
	}

	// Step 2: Split large groups (> 12 commits) by going one level deeper
	refined := make(map[string][]git.Commit)
	for dir, cs := range dirCommits {
		if len(cs) <= 12 {
			refined[dir] = cs
			continue
		}
		for _, c := range cs {
			deeper := dominantDirAtDepth(c.Files, 3)
			refined[deeper] = append(refined[deeper], c)
		}
	}

	// Step 3: Merge small groups (< 3 commits) into most similar larger group
	final := make(map[string][]git.Commit)
	var smallDirs []string
	for dir, cs := range refined {
		if len(cs) < 3 {
			smallDirs = append(smallDirs, dir)
		} else {
			final[dir] = cs
		}
	}

	// Sort smallDirs for deterministic merging
	sort.Strings(smallDirs)

	for _, smallDir := range smallDirs {
		bestMatch := ""
		bestPrefixLen := 0
		for dir := range final {
			pl := sharedPrefixLen(smallDir, dir)
			if pl > bestPrefixLen {
				bestPrefixLen = pl
				bestMatch = dir
			}
		}
		if bestMatch != "" && bestPrefixLen > 0 {
			final[bestMatch] = append(final[bestMatch], refined[smallDir]...)
		} else {
			final["other"] = append(final["other"], refined[smallDir]...)
		}
	}

	// If no groups survived (all were small), put everything in one group
	if len(final) == 0 {
		hashes := make([]string, len(commits))
		for i, c := range commits {
			h := c.Hash
			if len(h) > 8 {
				h = h[:8]
			}
			hashes[i] = h
		}
		return []topicGroup{{Topic: "all changes", Commits: hashes}}
	}

	// Step 4: Build topicGroup slice
	var groups []topicGroup
	for dir, cs := range final {
		hashes := make([]string, len(cs))
		for i, c := range cs {
			h := c.Hash
			if len(h) > 8 {
				h = h[:8]
			}
			hashes[i] = h
		}
		groups = append(groups, topicGroup{Topic: dir, Commits: hashes})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Topic < groups[j].Topic
	})

	return groups
}

// planTopics groups commits by file path and optionally names them via LLM.
// This function never returns an error — the heuristic always produces a result.
func (g *BlogPostGenerator) planTopics(ctx context.Context, commits []git.Commit) []topicGroup {
	fmt.Printf("  Planning topics for %d commits...\n", len(commits))

	groups := groupByFilePath(commits)

	// Try to get better topic names from LLM (non-critical)
	groups = g.nameTopics(ctx, groups, commits)

	fmt.Printf("  Planned %d topic groups\n", len(groups))
	for _, tg := range groups {
		fmt.Printf("    - %s (%d commits)\n", tg.Topic, len(tg.Commits))
	}

	return groups
}

// nameTopics asks the LLM to give each group a blog-post-worthy name.
// If the LLM call fails, returns the original groups with directory-based names.
func (g *BlogPostGenerator) nameTopics(ctx context.Context, groups []topicGroup, commits []git.Commit) []topicGroup {
	if g.LLM == nil {
		return groups
	}

	// Build a lookup from short hash to commit subject
	subjectByHash := make(map[string]string)
	for _, c := range commits {
		h := c.Hash
		if len(h) > 8 {
			h = h[:8]
		}
		subjectByHash[h] = c.Subject
	}

	// Build prompt listing each group's commits
	var b strings.Builder
	for i, grp := range groups {
		fmt.Fprintf(&b, "Group %d (%s, %d commits):\n", i+1, grp.Topic, len(grp.Commits))
		for _, h := range grp.Commits {
			fmt.Fprintf(&b, "- %s: %s\n", h, subjectByHash[h])
		}
		b.WriteString("\n")
	}

	resp, err := g.LLM.Generate(ctx, llm.Request{
		SystemPrompt: g.Templates.TopicPlanner,
		UserPrompt:   b.String(),
		MaxTokens:    1024,
		Temperature:  0.3,
	})
	if err != nil {
		log.Printf("  Warning: topic naming failed, using directory names: %v", err)
		return groups
	}

	resp = strings.TrimSpace(resp)
	if resp == "" {
		log.Printf("  Warning: topic naming returned empty response, using directory names")
		return groups
	}

	// Parse response: expect lines like "Group 1: Topic Name"
	named := make([]topicGroup, len(groups))
	copy(named, groups)

	lines := strings.Split(resp, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Group ") {
			continue
		}
		rest := strings.TrimPrefix(line, "Group ")
		colonIdx := strings.Index(rest, ":")
		if colonIdx < 0 {
			continue
		}
		numStr := strings.TrimSpace(rest[:colonIdx])
		name := strings.TrimSpace(rest[colonIdx+1:])

		var num int
		if _, err := fmt.Sscanf(numStr, "%d", &num); err != nil {
			continue
		}
		if num >= 1 && num <= len(named) && name != "" {
			named[num-1].Topic = name
		}
	}

	return named
}

// stripMarkdown removes markdown formatting from a line for pattern matching.
func stripMarkdown(s string) string {
	s = strings.TrimLeft(s, "#")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "*")
	s = strings.TrimSpace(s)
	return s
}

// stripMetaCommentary removes LLM preamble and trailing self-analysis from skill output.
func stripMetaCommentary(text string) string {
	lines := strings.Split(text, "\n")

	// Strip preamble: skip leading lines that look like LLM meta-commentary
	start := 0
	for start < len(lines) {
		trimmed := strings.TrimSpace(lines[start])
		if trimmed == "" {
			start++
			continue
		}
		lower := strings.ToLower(stripMarkdown(trimmed))
		if isPreambleLine(lower) {
			start++
			continue
		}
		break
	}

	// Strip trailing self-analysis: scan forward for first meta-commentary marker.
	// Once found, truncate everything from that line onward.
	// This handles multi-line meta blocks (marker + bullet lists + closing instruction).
	end := len(lines)
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(stripMarkdown(trimmed))
		if isTrailingMeta(lower) {
			end = i
			// If the preceding non-empty line is a separator (---), include it in the cut
			for j := i - 1; j > start; j-- {
				prev := strings.TrimSpace(lines[j])
				if prev == "" {
					continue
				}
				allDashes := strings.Trim(strings.ToLower(prev), "-=_ ")
				if allDashes == "" && len(prev) >= 3 {
					end = j
				}
				break
			}
			break
		}
	}

	if start >= end {
		return text // safety: don't strip everything
	}

	return strings.Join(lines[start:end], "\n")
}

func isPreambleLine(lower string) bool {
	preambles := []string{
		"draft rewrite",
		"revised version",
		"here is the rewrite",
		"here's the rewrite",
		"here is the revised",
		"here's the revised",
		"here is my rewrite",
		"here's my rewrite",
		"below is the rewrite",
		"below is the revised",
		"rewritten version",
		"edited version",
		"here is the edited",
		"here's the edited",
		"here is the updated",
		"here's the updated",
	}
	for _, p := range preambles {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	// Lines that are just dashes or equals (separators after preamble)
	allDashes := strings.Trim(lower, "-=_ ")
	if allDashes == "" && len(lower) >= 3 {
		return true
	}
	return false
}

func isTrailingMeta(lower string) bool {
	markers := []string{
		"what makes the below",
		"what makes the above",
		"what makes this",
		"here's what i changed",
		"here is what i changed",
		"changes i made",
		"changes made",
		"key changes",
		"what i changed",
		"notes on changes",
		"notes on the rewrite",
		"summary of changes",
		"now make it not obviously",
		"now make it not",
		"final rewrite",
	}
	for _, m := range markers {
		if strings.HasPrefix(lower, m) {
			return true
		}
	}
	// Bracketed meta-notes like "[The final rewrite..." or "[Note:..."
	if strings.HasPrefix(lower, "[") && !strings.HasPrefix(lower, "[!") {
		bracketMeta := []string{"[the ", "[note", "[edit", "[rewrite", "[revision", "[change", "[final"}
		for _, bm := range bracketMeta {
			if strings.HasPrefix(lower, bm) {
				return true
			}
		}
	}
	return false
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func extractSlug(content string) string {
	// Try to extract title from frontmatter
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "title") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				title := strings.Trim(strings.TrimSpace(parts[1]), "'\"")
				slug := strings.ToLower(title)
				slug = nonAlphanumeric.ReplaceAllString(slug, "-")
				slug = strings.Trim(slug, "-")
				if len(slug) > 60 {
					slug = slug[:60]
				}
				return slug
			}
		}
	}
	return "blog-post"
}
