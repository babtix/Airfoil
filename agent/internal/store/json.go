// Package store reads and writes the JSON files under data/.
//
// Every write is atomic: content goes to a temp file in the destination
// directory and is then renamed into place. A crashed run leaves the previous
// file intact rather than a half-written one.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadJSON decodes the file at path into a value of type T.
// A missing file is reported via os.IsNotExist on the returned error.
func ReadJSON[T any](path string) (T, error) {
	var v T
	b, err := os.ReadFile(path)
	if err != nil {
		return v, fmt.Errorf("store: read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("store: parse %s: %w", path, err)
	}
	return v, nil
}

// ReadJSONOr decodes the file at path, returning fallback if it does not exist.
// A file that exists but does not parse is still an error — silently discarding
// corrupt state would break idempotency.
func ReadJSONOr[T any](path string, fallback T) (T, error) {
	v, err := ReadJSON[T](path)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return fallback, nil
	}
	return v, err
}

// WriteJSON encodes v as indented JSON and writes it to path atomically,
// creating parent directories as needed.
func WriteJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("store: encode %s: %w", path, err)
	}
	return WriteFile(path, append(b, '\n'))
}

// WriteFile writes b to path atomically, creating parent directories as needed.
func WriteFile(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store: mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("store: temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()

	// On any failure past this point, remove the temp file rather than leaving
	// litter next to the real one.
	defer func() {
		if tmpName != "" {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("store: write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("store: sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store: close %s: %w", tmpName, err)
	}

	// Windows will not rename onto an existing file, so clear the way first.
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("store: replace %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("store: rename into %s: %w", path, err)
	}
	tmpName = "" // renamed; nothing to clean up

	return nil
}

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
