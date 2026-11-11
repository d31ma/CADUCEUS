package blobs

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

type AtomicFile struct {
	Path string
}

func (file AtomicFile) Write(body io.Reader, size int64) error {
	temporary, err := os.CreateTemp(filepath.Dir(file.Path), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	written, copyErr := io.Copy(temporary, io.LimitReader(body, size+1))
	err = errors.Join(copyErr, file.checkSize(written, size), temporary.Sync(), temporary.Close())
	if err != nil {
		return err
	}
	return os.Rename(temporary.Name(), file.Path)
}

func (AtomicFile) checkSize(written int64, size int64) error {
	if written != size {
		return SizeMismatch{Expected: size, Received: written}
	}
	return nil
}
