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

// SanitizeLLMOutput extracts clean markdown from LLM output by removing
// wrapper code fences that LLMs sometimes add around their entire response.
// Only strips fences when the output starts with ``` (after trimming), so
// embedded code blocks within the content are preserved.
func SanitizeLLMOutput(s string) string {
	trimmed := strings.TrimSpace(s)
	// Only strip if the output starts with a code fence (wrapper fence)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	// Skip the opening fence and language tag line (```markdown, ```md, etc.)
	after := trimmed[3:]
	if nlIdx := strings.Index(after, "\n"); nlIdx >= 0 {
		after = after[nlIdx+1:]
	}
	// Find closing fence at the end
	if endIdx := strings.LastIndex(after, "```"); endIdx >= 0 {
		return strings.TrimSpace(after[:endIdx])
	}
	return strings.TrimSpace(after)
}
