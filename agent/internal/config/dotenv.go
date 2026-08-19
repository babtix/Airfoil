package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// loadDotEnv reads a .env file into the process environment.
//
// Variables already set in the environment are never overwritten, so CI secrets
// always win over a stale local file. A missing .env is not an error — that is
// the normal case in CI.
//
// This is deliberately a small parser rather than a dependency: it handles
// KEY=value, optional `export` prefix, # comments, and single or double quoted
// values. It does not do interpolation.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config: open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		key, val, ok := parseDotEnvLine(sc.Text())
		if !ok {
			continue
		}
		if key == "" {
			return fmt.Errorf("config: %s:%d: malformed line", path, line)
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf("config: set %s: %w", key, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	return nil
}

// parseDotEnvLine splits one line into a key and value. ok is false for blank
// lines and comments; ok is true with an empty key for malformed lines.
func parseDotEnvLine(raw string) (key, val string, ok bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	name, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", true // malformed
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", true
	}

	value = strings.TrimSpace(value)
	switch {
	case len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		value = value[1 : len(value)-1]
	case len(value) >= 2 && strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`):
		value = value[1 : len(value)-1]
	default:
		// An unquoted trailing comment is not part of the value.
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
	}
	return name, value, true
}
