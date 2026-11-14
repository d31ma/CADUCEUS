package adapters

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"mesail.dev/packages/blobs"
)

func localBlobStore(t *testing.T) blobs.LocalFolder {
	return blobs.NewLocalFolder(t.TempDir(), "", blobs.NewLocalSigner(nil))
}

func TestBlobRawStoreWritesTheContentUnderTheKey(t *testing.T) {
	folder := localBlobStore(t)
	err := NewBlobRawStore(folder).Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := folder.Read(blobs.Key{DomainId: "example.com", Name: "inbound-abc"})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "raw" {
		t.Fatalf("unexpected content %q", content)
	}
}

func TestBlobRawStoreOverwritesTheSameKey(t *testing.T) {
	folder := localBlobStore(t)
	store := NewBlobRawStore(folder)
	store.Put(context.Background(), "example.com/inbound-abc", []byte("first"))
	store.Put(context.Background(), "example.com/inbound-abc", []byte("second"))
	reader, _ := folder.Read(blobs.Key{DomainId: "example.com", Name: "inbound-abc"})
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "second" {
		t.Fatalf("want the second copy, got %q", content)
	}
}

func TestBlobRawStoreRefusesParentFolderKeys(t *testing.T) {
	err := NewBlobRawStore(localBlobStore(t)).Put(context.Background(), "example.com/../../etc/passwd", []byte("x"))
	if !errors.Is(err, InvalidKey{}) || err.Error() == "" {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestBlobRawStoreRefusesKeysWithoutAName(t *testing.T) {
	err := NewBlobRawStore(localBlobStore(t)).Put(context.Background(), "example.com", []byte("x"))
	if !errors.Is(err, InvalidKey{}) {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestBlobRawStoreRefusesBadDomains(t *testing.T) {
	err := NewBlobRawStore(localBlobStore(t)).Put(context.Background(), "/etc/passwd", []byte("x"))
	if !errors.Is(err, InvalidKey{}) {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestBlobRawStoreRefusesBadKeysBeforeAnyWrite(t *testing.T) {
	folder := t.TempDir()
	err := NewBlobRawStore(blobs.NewLocalFolder(folder, "", blobs.NewLocalSigner(nil))).Put(context.Background(), "example.com/a/b", []byte("x"))
	entries, _ := os.ReadDir(folder)
	if !errors.Is(err, InvalidKey{}) || len(entries) != 0 {
		t.Fatalf("want InvalidKey and no file, got %v and %d entries", err, len(entries))
	}
}

func TestBlobRawStoreReportsAWriteFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-folder")
	os.WriteFile(file, []byte("x"), 0o600)
	err := NewBlobRawStore(blobs.NewLocalFolder(file, "", blobs.NewLocalSigner(nil))).Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err == nil {
		t.Fatal("want an error")
	}
}
