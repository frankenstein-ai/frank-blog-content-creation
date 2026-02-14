package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadTOML reads a flat key = "value" TOML file into a map.
// Returns an empty map (not an error) if the file does not exist.
// Handles comments (#), blank lines, and quoted values.
// Table headers ([section]) are silently skipped.
func LoadTOML(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()

	result := map[string]string{}
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

		// Strip surrounding quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		result[key] = val
	}
	return result, scanner.Err()
}
