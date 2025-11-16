package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandListFiles(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "volumes.txt")
	content := `
# comment
/path/Vol 01.epub

   /path/Vol 02.epub
`
	if err := os.WriteFile(list, []byte(content), 0o644); err != nil {
		t.Fatalf("write list: %v", err)
	}

	out, err := expandListFiles([]string{list})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	want := []string{"/path/Vol 01.epub", "/path/Vol 02.epub"}
	if len(out) != len(want) {
		t.Fatalf("got %d entries want %d", len(out), len(want))
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("entry %d = %q want %q", i, out[i], want[i])
		}
	}
}

func TestExpandListFilesMissing(t *testing.T) {
	if _, err := expandListFiles([]string{"/no/such/file"}); err == nil {
		t.Fatalf("expected error for missing file")
	}
}
