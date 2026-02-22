package hugo

import "strings"

// SplitFrontmatter splits a Hugo file into frontmatter (including +++ delimiters)
// and the body content after it.
func SplitFrontmatter(content string) (frontmatter, body string) {
	parts := strings.SplitN(content, "+++", 3)
	if len(parts) == 3 {
		frontmatter = "+++" + parts[1] + "+++\n"
		body = strings.TrimSpace(parts[2])
		return
	}
	return "", content
}

// StripFrontmatter removes +++ delimited TOML frontmatter from content.
func StripFrontmatter(content string) string {
	_, body := SplitFrontmatter(content)
	return body
}

// SanitizeLLMOutput extracts clean markdown from LLM output by removing code fences,
// preamble text, and trailing conversational text.
func SanitizeLLMOutput(s string) string {
	// If there's a code fence, extract its content
	if startIdx := strings.Index(s, "```"); startIdx >= 0 {
		after := s[startIdx+3:]
		// Skip the language tag line (```markdown, ```md, etc.)
		if nlIdx := strings.Index(after, "\n"); nlIdx >= 0 {
			after = after[nlIdx+1:]
		}
		// Find closing fence
		if endIdx := strings.LastIndex(after, "```"); endIdx >= 0 {
			return strings.TrimSpace(after[:endIdx])
		}
		return strings.TrimSpace(after)
	}
	return strings.TrimSpace(s)
}
