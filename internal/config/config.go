package config

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type Config struct {
	LLMProvider string
	LLMModel    string
	APIKey      string

	SourceRepo   string
	OutputDir    string
	NotebooksDir string
	MemosDir     string
	BlogDir      string

	StateDB string
	Period  string
	DryRun  bool
}

// Load resolves config from CLI flags then env vars then defaults.
func Load(cmd *cobra.Command) (*Config, error) {
	cfg := &Config{}

	cfg.LLMProvider = flagOrEnv(cmd, "llm-provider", "FRANK_LLM_PROVIDER")
	cfg.LLMModel = flagOrEnv(cmd, "llm-model", "FRANK_LLM_MODEL")
	cfg.StateDB = flagOrEnvDefault(cmd, "state-db", "FRANK_STATE_DB", ".frank-state.db")
	cfg.DryRun, _ = cmd.Flags().GetBool("dry-run")

	cfg.SourceRepo = flagOrEnv(cmd, "source-repo", "FRANK_SOURCE_REPO")
	cfg.OutputDir = flagOrEnv(cmd, "output-dir", "FRANK_OUTPUT_DIR")
	cfg.NotebooksDir = flagOrEnv(cmd, "notebooks-dir", "FRANK_NOTEBOOKS_DIR")
	cfg.MemosDir = flagOrEnv(cmd, "memos-dir", "FRANK_MEMOS_DIR")
	cfg.BlogDir = flagOrEnv(cmd, "output-dir", "FRANK_BLOG_DIR")
	cfg.Period = flagOrEnvDefault(cmd, "period", "", "week")

	// Resolve API key based on provider
	switch cfg.LLMProvider {
	case "openai":
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		cfg.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	case "openrouter":
		cfg.APIKey = os.Getenv("OPENROUTER_API_KEY")
	}

	return cfg, nil
}

// Validate checks that required fields are set for a given command context.
func (c *Config) ValidateForGenerate() error {
	if c.LLMProvider == "" {
		return fmt.Errorf("--llm-provider or FRANK_LLM_PROVIDER is required")
	}
	if c.LLMProvider != "openai" && c.LLMProvider != "anthropic" && c.LLMProvider != "ollama" && c.LLMProvider != "openrouter" {
		return fmt.Errorf("llm-provider must be 'openai', 'anthropic', 'ollama', or 'openrouter', got %q", c.LLMProvider)
	}
	if c.LLMProvider != "ollama" && c.APIKey == "" {
		return fmt.Errorf("API key not set: set OPENAI_API_KEY, ANTHROPIC_API_KEY, or OPENROUTER_API_KEY")
	}
	return nil
}

func flagOrEnv(cmd *cobra.Command, flag, env string) string {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		return v
	}
	return os.Getenv(env)
}

func flagOrEnvDefault(cmd *cobra.Command, flag, env, def string) string {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		return v
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}
