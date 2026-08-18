package store

import (
	"os"
	"path/filepath"
	"testing"
)

type record struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestWriteJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	want := record{Name: "airfoil", Count: 42}

	if err := WriteJSON(path, want); err != nil {
		t.Fatalf("WriteJSON() = %v", err)
	}
	got, err := ReadJSON[record](path)
	if err != nil {
		t.Fatalf("ReadJSON() = %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestWriteJSONCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "c", "out.json")

	if err := WriteJSON(path, record{Name: "x"}); err != nil {
		t.Fatalf("WriteJSON() = %v", err)
	}
	if !Exists(path) {
		t.Error("file was not created")
	}
}

// Overwriting must work on Windows, where rename onto an existing file fails
// unless the destination is removed first.
func TestWriteJSONOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")

	if err := WriteJSON(path, record{Name: "first", Count: 1}); err != nil {
		t.Fatalf("first WriteJSON() = %v", err)
	}
	if err := WriteJSON(path, record{Name: "second", Count: 2}); err != nil {
		t.Fatalf("second WriteJSON() = %v", err)
	}

	got, err := ReadJSON[record](path)
	if err != nil {
		t.Fatalf("ReadJSON() = %v", err)
	}
	if want := (record{Name: "second", Count: 2}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// A completed write must leave no temp files behind, or data/ fills with litter.
func TestWriteJSONLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()

	for range 3 {
		if err := WriteJSON(filepath.Join(dir, "out.json"), record{Name: "x"}); err != nil {
			t.Fatalf("WriteJSON() = %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("directory holds %v, want only out.json", names)
	}
}

func TestReadJSONOrMissingFileReturnsFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	fallback := record{Name: "fallback"}

	got, err := ReadJSONOr(path, fallback)
	if err != nil {
		t.Fatalf("ReadJSONOr() = %v, want nil", err)
	}
	if got != fallback {
		t.Errorf("got %+v, want the fallback %+v", got, fallback)
	}
}

// A corrupt file must not be silently replaced by the fallback — discarding
// state.json on a parse error would break idempotency (R7).
func TestReadJSONOrCorruptFileIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadJSONOr(path, record{Name: "fallback"}); err == nil {
		t.Fatal("ReadJSONOr() = nil, want a parse error")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	if Exists(path) {
		t.Error("Exists() = true before the file was written")
	}
	if err := WriteJSON(path, record{}); err != nil {
		t.Fatal(err)
	}
	if !Exists(path) {
		t.Error("Exists() = false after the file was written")
	}
}
