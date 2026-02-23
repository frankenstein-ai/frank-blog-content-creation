package generator

import (
	"strings"
	"testing"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/hugo"
)

// runHumanizerPipeline replicates the exact production pipeline from blogposts.go:188-190.
func runHumanizerPipeline(input string) string {
	processed := hugo.SanitizeLLMOutput(input)
	processed = stripMetaCommentary(processed)
	return strings.TrimSpace(processed)
}

func TestHumanizerPipelineIntegration(t *testing.T) {
	cleanContent := "## Building a Hardened File Sharing Service\n\nYesterday I spent most of my day wrestling with redirect chains and atomic downloads.\nThe core problem was deceptively simple: users kept getting 404s when downloading large files.\n\nAfter digging into the nginx logs, I noticed the redirect chain was bouncing requests\nthrough three different endpoints before finally hitting the storage backend.\n\n### The Fix\n\nI collapsed the redirect chain into a single hop and added atomic download support.\nThis meant reworking the download handler to stream directly from the storage layer."

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean output passes through",
			input:    cleanContent,
			expected: cleanContent,
		},
		{
			name:     "code fence wrapped output",
			input:    "```markdown\n" + cleanContent + "\n```",
			expected: cleanContent,
		},
		{
			name: "preamble inside code fence",
			input: "```markdown\n**Draft rewrite**\n---\n\n" + cleanContent + "\n```",
			expected: cleanContent,
		},
		{
			name: "trailing meta-commentary block",
			input: cleanContent + "\n\n**What makes this obviously AI generated?**\n- Repetitive sentence structures\n- Over-formal tone\n- Heavy use of bullet lists\n- Absence of first-person perspective\n\nNow make it not obviously AI generated",
			expected: cleanContent,
		},
		{
			name: "full realistic output with all issues",
			input: "```markdown\n## Here is the rewrite:\n\n" + cleanContent +
				"\n\n---\n**What makes the above so obviously AI generated?**\n- Formulaic structure\n- Lacks personal voice\n- Too many transition words\n\nKey changes made:\n- Added first-person voice\n- Varied sentence length\n- Removed formal phrasing\n```",
			expected: cleanContent,
		},
		{
			name: "content with embedded code blocks preserved",
			input: "```markdown\n" + "## Fixing the Download Handler\n\nHere's what the original handler looked like:\n\n```go\nfunc handleDownload(w http.ResponseWriter, r *http.Request) {\n    http.Redirect(w, r, \"/storage/\" + r.URL.Path, 302)\n}\n```\n\nThe fix was straightforward — stream directly from storage:\n\n```go\nfunc handleDownload(w http.ResponseWriter, r *http.Request) {\n    file, err := storage.Open(r.URL.Path)\n    if err != nil {\n        http.Error(w, \"not found\", 404)\n        return\n    }\n    io.Copy(w, file)\n}\n```\n\nMuch simpler and no more redirect chains." + "\n```",
			expected: "## Fixing the Download Handler\n\nHere's what the original handler looked like:\n\n```go\nfunc handleDownload(w http.ResponseWriter, r *http.Request) {\n    http.Redirect(w, r, \"/storage/\" + r.URL.Path, 302)\n}\n```\n\nThe fix was straightforward — stream directly from storage:\n\n```go\nfunc handleDownload(w http.ResponseWriter, r *http.Request) {\n    file, err := storage.Open(r.URL.Path)\n    if err != nil {\n        http.Error(w, \"not found\", 404)\n        return\n    }\n    io.Copy(w, file)\n}\n```\n\nMuch simpler and no more redirect chains.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := runHumanizerPipeline(tt.input)
			if got != tt.expected {
				t.Errorf("runHumanizerPipeline() mismatch\n got: %q\nwant: %q", got, tt.expected)
			}
		})
	}
}

func TestStripMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain text", "hello world", "hello world"},
		{"heading level 2", "## Draft rewrite", "Draft rewrite"},
		{"heading level 3", "### Some heading", "Some heading"},
		{"bold markers", "**Draft rewrite**", "Draft rewrite"},
		{"italic markers", "*emphasis*", "emphasis"},
		{"heading and bold", "## **Draft rewrite**", "Draft rewrite"},
		{"only whitespace", "   ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdown(tt.input)
			if got != tt.expected {
				t.Errorf("stripMarkdown(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsPreambleLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"draft rewrite", "draft rewrite", true},
		{"here is the rewrite", "here is the rewrite:", true},
		{"revised version", "here's the revised version", true},
		{"below is the rewrite", "below is the rewrite", true},
		{"edited version", "edited version of the post", true},
		{"separator dashes", "---", true},
		{"separator equals", "===", true},
		{"too short separator", "--", false},
		{"regular text", "the quick brown fox", false},
		{"partial match", "here we go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPreambleLine(tt.input)
			if got != tt.expected {
				t.Errorf("isPreambleLine(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsTrailingMeta(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"what makes below", "what makes the below so obviously ai generated?", true},
		{"what makes above", "what makes the above so obviously ai generated?", true},
		{"now make it not", "now make it not obviously ai generated", true},
		{"final rewrite", "final rewrite", true},
		{"key changes", "key changes made in this revision", true},
		{"summary of changes", "summary of changes", true},
		{"heres what i changed", "here's what i changed", true},
		{"bracketed note", "[note: this was rewritten]", true},
		{"bracketed edit", "[edit: improved flow]", true},
		{"markdown callout preserved", "[!note] this is important", false},
		{"numbered reference", "[1] first item", false},
		{"regular paragraph", "this is a regular paragraph", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTrailingMeta(tt.input)
			if got != tt.expected {
				t.Errorf("isTrailingMeta(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestStripMetaCommentary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean text no changes",
			input:    "This is a great blog post.\n\nIt has multiple paragraphs.",
			expected: "This is a great blog post.\n\nIt has multiple paragraphs.",
		},
		{
			name:     "preamble only plain text",
			input:    "Draft rewrite\n---\n\nThis is the real content.\n\nSecond paragraph.",
			expected: "This is the real content.\n\nSecond paragraph.",
		},
		{
			name:     "preamble with bold markdown",
			input:    "**Draft rewrite**\n\nThis is the real content.\n\nSecond paragraph.",
			expected: "This is the real content.\n\nSecond paragraph.",
		},
		{
			name:     "preamble with heading markdown",
			input:    "## Draft rewrite\n---\n\nThis is the real content.\n\nSecond paragraph.",
			expected: "This is the real content.\n\nSecond paragraph.",
		},
		{
			name:     "trailing meta only",
			input:    "This is the real content.\n\nSecond paragraph.\n\n**What makes the below so obviously AI generated?**\n- Repetitive structure\n- Too many lists\n- Formulaic tone",
			expected: "This is the real content.\n\nSecond paragraph.\n",
		},
		{
			name:     "both preamble and trailing meta",
			input:    "Here is the rewrite:\n\nThis is the real content.\n\nSecond paragraph.\n\nWhat makes the above so obviously AI generated?\n- Too formal\n- Lacks voice",
			expected: "This is the real content.\n\nSecond paragraph.\n",
		},
		{
			name:     "trailing meta preceded by separator",
			input:    "This is the real content.\n\nSecond paragraph.\n\n---\nKey changes:\n- Fixed tone\n- Removed jargon",
			expected: "This is the real content.\n\nSecond paragraph.\n",
		},
		{
			name:     "preamble with colon and separator",
			input:    "Here is the rewrite:\n\n---\n\nThis is the real content.\n\nSecond paragraph.",
			expected: "This is the real content.\n\nSecond paragraph.",
		},
		{
			name:     "safety all preamble",
			input:    "Draft rewrite\n---",
			expected: "Draft rewrite\n---",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMetaCommentary(tt.input)
			if got != tt.expected {
				t.Errorf("stripMetaCommentary() mismatch\n got: %q\nwant: %q", got, tt.expected)
			}
		})
	}
}
