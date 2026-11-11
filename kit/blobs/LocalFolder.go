package blobs

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type LocalFolder struct {
	root    string
	baseUrl string
	signer  LocalSigner
}

func NewLocalFolder(root string, baseUrl string, signer LocalSigner) LocalFolder {
	return LocalFolder{root: root, baseUrl: baseUrl, signer: signer}
}

func (folder LocalFolder) SignUpload(key Key, upload UploadTerms, expiresAt time.Time) (string, error) {
	grant := Grant{Method: http.MethodPut, Key: key, ExpiresAt: expiresAt.Unix(), MediaType: upload.MediaType, Size: upload.Size}
	return folder.signedUrl(grant)
}

func (folder LocalFolder) SignDownload(key Key, download DownloadTerms, expiresAt time.Time) (string, error) {
	grant := Grant{Method: http.MethodGet, Key: key, ExpiresAt: expiresAt.Unix(), MediaType: download.MediaType, FileName: download.FileName}
	return folder.signedUrl(grant)
}

func (folder LocalFolder) Read(key Key) (io.ReadCloser, error) {
	path, err := folder.path(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, folder.openFailure(err, key)
	}
	return file, nil
}

func (folder LocalFolder) Delete(key Key) error {
	path, err := folder.path(key)
	if err != nil {
		return err
	}
	return folder.removed(os.Remove(path))
}

func (folder LocalFolder) Write(key Key, body io.Reader, size int64) error {
	path, err := folder.path(key)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(path), folder.folderMode())
	if err != nil {
		return err
	}
	return AtomicFile{Path: path}.Write(body, size)
}

func (folder LocalFolder) Grant(method string, key Key, query map[string][]string, now time.Time) (Grant, error) {
	err := KeyRules{}.Check(key)
	if err != nil {
		return Grant{}, err
	}
	return folder.signer.Grant(method, key, query, now)
}

func (folder LocalFolder) signedUrl(grant Grant) (string, error) {
	err := KeyRules{}.Check(grant.Key)
	if err != nil {
		return "", err
	}
	return folder.baseUrl + "/" + grant.Key.Path() + "?" + folder.signer.Query(grant).Encode(), nil
}

func (folder LocalFolder) path(key Key) (string, error) {
	err := KeyRules{}.Check(key)
	return filepath.Join(folder.root, key.DomainId, key.Name), err
}

func (LocalFolder) openFailure(err error, key Key) error {
	if errors.Is(err, fs.ErrNotExist) {
		return NotFound{Path: key.Path()}
	}
	return err
}

func (LocalFolder) removed(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (LocalFolder) folderMode() os.FileMode {
	return 0o700
}
