package search

import (
	"os"
	"path/filepath"
)

var knownCollectionsBase = filepath.Join(os.TempDir(), "mesail-kit-tests")

func isolateKnownCollections() {
	os.MkdirAll(knownCollectionsBase, 0o700)
	folder, _ := os.MkdirTemp(knownCollectionsBase, "search")
	os.Setenv("TMPDIR", folder)
}
