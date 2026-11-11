package oidc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

type JsonFileCache struct {
	Folder string
}

func (cache JsonFileCache) Load(key string, into any) {
	raw, _ := os.ReadFile(cache.path(key))
	_ = json.Unmarshal(raw, into)
}

func (cache JsonFileCache) Store(key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return AtomicFile{}.Write(cache.path(key), raw)
}

func (cache JsonFileCache) path(key string) string {
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(cache.Folder, hex.EncodeToString(digest[:])+".json")
}
