package config

import (
	"bufio"
	"os"
	"strings"
)

// TOMLConfig holds parsed values from .frank.toml.
type TOMLConfig struct {
	Values map[string]string
	Skills []string
}

// LoadTOML reads a flat key = "value" TOML file.
// Returns an empty config (not an error) if the file does not exist.
// Handles comments (#), blank lines, quoted values, and simple arrays.
// Table headers ([section]) are silently skipped.
func LoadTOML(path string) (*TOMLConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TOMLConfig{Values: map[string]string{}}, nil
		}
		return nil, err
	}
	defer f.Close()

	cfg := &TOMLConfig{Values: map[string]string{}}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines, comments, and table headers
		if line == "" || line[0] == '#' || line[0] == '[' {
			continue
		}

		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])

		// Parse array values: skills = ["humanizer", "other"]
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			inner := val[1 : len(val)-1]
			if key == "skills" {
				cfg.Skills = parseStringArray(inner)
			}
			continue
		}

		// Strip surrounding quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		cfg.Values[key] = val
	}
	return cfg, scanner.Err()
}

// parseStringArray parses the inner content of a TOML array of strings.
// e.g. `"humanizer", "other"` -> ["humanizer", "other"]
func parseStringArray(s string) []string {
	var result []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		// Strip quotes
		if len(item) >= 2 {
			if (item[0] == '"' && item[len(item)-1] == '"') ||
				(item[0] == '\'' && item[len(item)-1] == '\'') {
				item = item[1 : len(item)-1]
			}
		}
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
