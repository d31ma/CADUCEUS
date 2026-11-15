package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestVendoredKitMatchesKit(t *testing.T) {
	vendored := filepath.Join("vendor", "mesail.dev", "packages")
	files, err := filepath.Glob(filepath.Join(vendored, "*", "*.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no vendored kit files: %v", err)
	}
	for _, file := range files {
		relative, _ := filepath.Rel(vendored, file)
		copied, _ := os.ReadFile(file)
		original, err := os.ReadFile(filepath.Join("..", "..", "kit", relative))
		if err != nil || !bytes.Equal(copied, original) {
			t.Fatalf("vendor/mesail.dev/packages/%s differs from kit/%s; run scripts/kit-sync.sh", relative, relative)
		}
	}
}
