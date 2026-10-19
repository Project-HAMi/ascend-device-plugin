package server

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFileKeepsOpenOldLibrary(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	source, destination := filepath.Join(directory, "bundle.so"), filepath.Join(directory, "installed.so")
	if err := os.WriteFile(source, []byte("new runtime"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old runtime"), 0755); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = old.Close() }()
	if err := copyFile(source, destination); err != nil {
		t.Fatal(err)
	}
	previous, err := io.ReadAll(old)
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(previous) != "old runtime" || string(current) != "new runtime" {
		t.Fatalf("old descriptor=%q, new path=%q", previous, current)
	}
}

func TestCopyFileFailurePreservesInstalledLibrary(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	destination := filepath.Join(directory, "installed.so")
	if err := os.WriteFile(destination, []byte("installed runtime"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{filepath.Join(directory, "missing.so"), directory} {
		if err := copyFile(source, destination); err == nil {
			t.Fatalf("invalid source accepted: %s", source)
		}
		content, err := os.ReadFile(destination)
		if err != nil || string(content) != "installed runtime" {
			t.Fatalf("installed library changed: %q, %v", content, err)
		}
	}
	staged, err := filepath.Glob(filepath.Join(directory, ".installed.so.tmp-*"))
	if err != nil || len(staged) != 0 {
		t.Fatalf("staged files left behind: %v, %v", staged, err)
	}
}
