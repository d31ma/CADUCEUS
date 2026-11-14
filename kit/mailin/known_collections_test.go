package mailin

import (
	"os"
	"path/filepath"
)

var knownCollectionsBase = filepath.Join(os.TempDir(), "mesail-kit-tests")

func isolateKnownCollections() {
	os.MkdirAll(knownCollectionsBase, 0o700)
	folder, _ := os.MkdirTemp(knownCollectionsBase, "mailin")
	os.Setenv("TMPDIR", folder)
}
