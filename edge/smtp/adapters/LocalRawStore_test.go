package adapters

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStoreWritesTheContentUnderTheKey(t *testing.T) {
	folder := t.TempDir()
	err := NewLocalRawStore(folder).Put(context.Background(), "example.com/inbound/abc.eml", []byte("raw"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(folder, "example.com", "inbound", "abc.eml"))
	if err != nil || string(content) != "raw" {
		t.Fatalf("unexpected content %q %v", content, err)
	}
}

func TestStoreOverwritesTheSameKey(t *testing.T) {
	folder := t.TempDir()
	store := NewLocalRawStore(folder)
	store.Put(context.Background(), "example.com/inbound/abc.eml", []byte("first"))
	store.Put(context.Background(), "example.com/inbound/abc.eml", []byte("second"))
	entries, _ := os.ReadDir(filepath.Join(folder, "example.com", "inbound"))
	content, _ := os.ReadFile(filepath.Join(folder, "example.com", "inbound", "abc.eml"))
	if len(entries) != 1 || string(content) != "second" {
		t.Fatalf("want one file with the second copy, got %d files and %q", len(entries), content)
	}
}

func TestStoreRefusesParentFolderKeys(t *testing.T) {
	err := NewLocalRawStore(t.TempDir()).Put(context.Background(), "example.com/../../etc/passwd", []byte("x"))
	if !errors.Is(err, InvalidKey{}) {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestStoreRefusesAbsoluteKeys(t *testing.T) {
	err := NewLocalRawStore(t.TempDir()).Put(context.Background(), "/etc/passwd", []byte("x"))
	if !errors.Is(err, InvalidKey{}) || err.Error() == "" {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestStoreRefusesEmptyKeys(t *testing.T) {
	err := NewLocalRawStore(t.TempDir()).Put(context.Background(), "", []byte("x"))
	if !errors.Is(err, InvalidKey{}) {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestStoreReportsAFolderItCannotCreate(t *testing.T) {
	folder := t.TempDir()
	os.WriteFile(filepath.Join(folder, "example.com"), []byte("a file, not a folder"), 0o600)
	err := NewLocalRawStore(folder).Put(context.Background(), "example.com/inbound/abc.eml", []byte("x"))
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestStoreReportsAFailedRenameAndLeavesNoTemporaryFile(t *testing.T) {
	folder := t.TempDir()
	target := filepath.Join(folder, "example.com", "inbound", "abc.eml")
	os.MkdirAll(filepath.Join(target, "inside"), 0o750)
	err := NewLocalRawStore(folder).Put(context.Background(), "example.com/inbound/abc.eml", []byte("x"))
	entries, _ := os.ReadDir(filepath.Join(folder, "example.com", "inbound"))
	if err == nil || len(entries) != 1 {
		t.Fatalf("want an error and only the blocking folder, got %v and %d entries", err, len(entries))
	}
}

func TestStoreReportsAReadOnlyFolder(t *testing.T) {
	folder := t.TempDir()
	inbound := filepath.Join(folder, "example.com", "inbound")
	os.MkdirAll(inbound, 0o750)
	os.Chmod(inbound, 0o500)
	t.Cleanup(func() { os.Chmod(inbound, 0o750) })
	err := NewLocalRawStore(folder).Put(context.Background(), "example.com/inbound/abc.eml", []byte("x"))
	if err == nil {
		t.Fatal("want an error")
	}
}
