package realtime

import (
	"os"
	"path/filepath"
)

var knownCollectionsBase = filepath.Join(os.TempDir(), "mesail-kit-tests")

func isolateKnownCollections() {
	os.MkdirAll(knownCollectionsBase, 0o700)
	folder, _ := os.MkdirTemp(knownCollectionsBase, "realtime")
	os.Setenv("TMPDIR", folder)
}
