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

	HugoDir string

	StateDB string
	Period  string
	DryRun  bool
}

// Load resolves config from CLI flags > env vars > .frank.toml > defaults.
func Load(cmd *cobra.Command) (*Config, error) {
	toml, err := LoadTOML(".frank.toml")
	if err != nil {
		return nil, fmt.Errorf("reading .frank.toml: %w", err)
	}

	cfg := &Config{}

	cfg.LLMProvider = flagOrEnvOrToml(cmd, "llm-provider", "FRANK_LLM_PROVIDER", toml["llm_provider"])
	cfg.LLMModel = flagOrEnvOrToml(cmd, "llm-model", "FRANK_LLM_MODEL", toml["llm_model"])
	cfg.StateDB = flagOrEnvOrTomlDefault(cmd, "state-db", "FRANK_STATE_DB", toml["state_db"], ".frank-state.db")
	cfg.DryRun, _ = cmd.Flags().GetBool("dry-run")

	cfg.HugoDir = flagOrEnvOrToml(cmd, "hugo-dir", "FRANK_HUGO_DIR", toml["hugo_dir"])
	cfg.Period = flagOrEnvOrTomlDefault(cmd, "period", "", toml["period"], "week")

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

// ValidateForGenerate checks that required fields are set for generation.
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

func flagOrEnvOrToml(cmd *cobra.Command, flag, env, tomlVal string) string {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		return v
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	return tomlVal
}

func flagOrEnvOrTomlDefault(cmd *cobra.Command, flag, env, tomlVal, def string) string {
	if cmd.Flags().Changed(flag) {
		v, _ := cmd.Flags().GetString(flag)
		return v
	}
	if v := os.Getenv(env); v != "" {
		return v
	}
	if tomlVal != "" {
		return tomlVal
	}
	return def
}
