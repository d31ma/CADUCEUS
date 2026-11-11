import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// @Delegate
// @NeedsAll()
type BlobStoreDelegate struct {
	settings *SettingsDelegate
}

func (BlobStoreDelegate) folderMode() os.FileMode { return 0o700 }

func (BlobStoreDelegate) fileMode() os.FileMode { return 0o600 }

func (blobs *BlobStoreDelegate) Put(domainId string, key string, data []byte) error {
	path, err := blobs.path(domainId, key)
	return blobs.write(path, data, err)
}

func (blobs *BlobStoreDelegate) Get(domainId string, key string) ([]byte, error) {
	path, err := blobs.path(domainId, key)
	return blobs.read(path, err)
}

func (blobs *BlobStoreDelegate) path(domainId string, key string) (string, error) {
	root, rootErr := blobs.root()
	return filepath.Join(root, filepath.FromSlash(key)), errors.Join(rootErr, blobs.check(domainId, key))
}

func (blobs *BlobStoreDelegate) root() (string, error) {
	folder := blobs.settings.Settings().BlobFolder
	if folder == "" {
		return "", errors.New("mail-out has no blob folder")
	}
	return folder, nil
}

func (BlobStoreDelegate) check(domainId string, key string) error {
	pattern := regexp.MustCompile(`^[a-z0-9.-]+/[A-Za-z0-9_-]{1,128}$`)
	if !pattern.MatchString(key) || strings.Contains(key, "..") || !strings.HasPrefix(key, strings.ToLower(domainId)+"/") {
		return errors.New("the blob key is not inside the domain folder")
	}
	return nil
}

func (blobs *BlobStoreDelegate) write(path string, data []byte, failure error) error {
	if failure != nil {
		return failure
	}
	return errors.Join(os.MkdirAll(filepath.Dir(path), blobs.folderMode()), os.WriteFile(path, data, blobs.fileMode()))
}

func (BlobStoreDelegate) read(path string, failure error) ([]byte, error) {
	if failure != nil {
		return nil, failure
	}
	return os.ReadFile(path)
}
