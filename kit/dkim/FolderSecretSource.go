package dkim

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type FolderSecretSource struct {
	Root string
}

func (folder FolderSecretSource) Secret(name string) ([]byte, error) {
	err := keyNames{}.checkSecret(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(folder.path(name))
	if err != nil {
		return nil, folder.readError(name, err)
	}
	return data, nil
}

func (FolderSecretSource) extension() string {
	return ".pem"
}

func (folder FolderSecretSource) path(name string) string {
	return filepath.Join(folder.Root, filepath.FromSlash(name)+folder.extension())
}

func (FolderSecretSource) readError(name string, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return MissingSecret{Name: name}
	}
	return err
}
