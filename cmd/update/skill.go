package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/frankenstein-ai/frank-blog-content-generator/internal/config"
	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill <name>",
	Short: "Download a skill definition from its upstream URL",
	Long:  "Re-download a skill's markdown definition from the URL configured in .frank.toml (skill_url_<name>).",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillUpdate,
}

func runSkillUpdate(cmd *cobra.Command, args []string) error {
	name := args[0]

	toml, err := config.LoadTOML(".frank.toml")
	if err != nil {
		return fmt.Errorf("reading .frank.toml: %w", err)
	}

	urlKey := "skill_url_" + name
	url := toml.Values[urlKey]
	if url == "" {
		return fmt.Errorf("no URL configured for skill %q (set %s in .frank.toml)", name, urlKey)
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading skill %q: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading skill %q: HTTP %d", name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response for skill %q: %w", name, err)
	}

	skillDir := "skills"
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}

	outPath := filepath.Join(skillDir, name+".md")
	if err := os.WriteFile(outPath, body, 0644); err != nil {
		return fmt.Errorf("writing skill file: %w", err)
	}

	fmt.Printf("Updated skill %q (%d bytes) -> %s\n", name, len(body), outPath)
	return nil
}
