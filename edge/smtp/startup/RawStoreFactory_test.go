package startup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRawStoreFactoryWritesIntoTheLocalFolder(t *testing.T) {
	settings := baseSettings(t)
	err := RawStoreFactory{}.Store(settings).Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	content, readError := os.ReadFile(filepath.Join(settings.RawStoreFolder, "example.com", "inbound-abc"))
	if err != nil || readError != nil || string(content) != "raw" {
		t.Fatalf("unexpected result %q %v %v", content, err, readError)
	}
}
