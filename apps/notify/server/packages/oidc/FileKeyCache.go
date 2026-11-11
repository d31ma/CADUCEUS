package oidc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type FileKeyCache struct {
	Folder string
}

func (cache FileKeyCache) Load(jwksUrl string) CachedKeySet {
	cached := CachedKeySet{}
	raw, _ := os.ReadFile(cache.path(jwksUrl))
	_ = json.Unmarshal(raw, &cached)
	return cached
}

func (cache FileKeyCache) Store(jwksUrl string, cached CachedKeySet) error {
	raw, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	err = os.MkdirAll(cache.Folder, cache.folderMode())
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(cache.Folder, "keys-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), cache.path(jwksUrl))
}

func (cache FileKeyCache) path(jwksUrl string) string {
	digest := sha256.Sum256([]byte(jwksUrl))
	return filepath.Join(cache.Folder, hex.EncodeToString(digest[:])+".json")
}

func (FileKeyCache) folderMode() os.FileMode {
	return 0o700
}
