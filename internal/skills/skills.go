package skills

import (
	"fmt"
	"os"
	"path/filepath"
)

type Skill struct {
	Name   string
	Prompt string
}

// Load reads skill files from dir for the given names.
// Each name maps to dir/name.md — the entire file content is used as the LLM system prompt.
func Load(dir string, names []string) ([]Skill, error) {
	var result []Skill
	for _, name := range names {
		path := filepath.Join(dir, name+".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading skill %q: %w", name, err)
		}
		result = append(result, Skill{
			Name:   name,
			Prompt: string(data),
		})
	}
	return result, nil
}
