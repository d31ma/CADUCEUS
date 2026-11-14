package collections

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
)

type KnownCollections struct {
	Folder string
}

func (known KnownCollections) Exists(store CollectionStore, name string) (bool, error) {
	return known.checked(store, name, known.marked(name))
}

func (known KnownCollections) Ensure(store CollectionStore, name string) error {
	exists, err := known.Exists(store, name)
	if err != nil {
		return err
	}
	return known.createUnless(store, name, exists)
}

func (known KnownCollections) Read(store CollectionStore, name string, read func() error) error {
	exists, err := known.Exists(store, name)
	if err != nil {
		return err
	}
	return known.readWhen(store, name, exists, read)
}

func (known KnownCollections) Write(store CollectionStore, name string, write func() error) error {
	err := known.Ensure(store, name)
	if err != nil {
		return err
	}
	return known.afterWrite(store, name, write, write())
}

func (known KnownCollections) ReadFailure(store CollectionStore, name string, failure error) error {
	return known.afterRead(store, name, failure)
}

func (known KnownCollections) Forget(name string) {
	os.Remove(known.markerPath(name))
}

func (known KnownCollections) ForgetAll() {
	os.RemoveAll(known.folder())
}

func (known KnownCollections) checked(store CollectionStore, name string, marked bool) (bool, error) {
	if marked {
		return true, nil
	}
	return known.asked(store, name)
}

func (known KnownCollections) asked(store CollectionStore, name string) (bool, error) {
	exists, err := store.CollectionExists(name)
	if err != nil {
		return false, err
	}
	known.remember(name, exists)
	return exists, nil
}

func (known KnownCollections) remember(name string, exists bool) {
	if exists {
		known.mark(name)
	}
}

func (known KnownCollections) mark(name string) {
	os.MkdirAll(known.folder(), known.folderMode())
	os.WriteFile(known.markerPath(name), nil, known.markerMode())
}

func (known KnownCollections) marked(name string) bool {
	_, err := os.Stat(known.markerPath(name))
	return err == nil
}

func (known KnownCollections) createUnless(store CollectionStore, name string, exists bool) error {
	if exists {
		return nil
	}
	return known.create(store, name)
}

func (known KnownCollections) create(store CollectionStore, name string) error {
	failure := store.CreateCollection(name)
	known.remember(name, failure == nil)
	return known.unlessCreatedElsewhere(store, name, failure)
}

func (known KnownCollections) unlessCreatedElsewhere(store CollectionStore, name string, failure error) error {
	if failure == nil {
		return nil
	}
	return known.recheck(store, name, failure)
}

func (known KnownCollections) recheck(store CollectionStore, name string, failure error) error {
	exists, _ := known.asked(store, name)
	return known.failureUnless(exists, failure)
}

func (known KnownCollections) readWhen(store CollectionStore, name string, exists bool, read func() error) error {
	if !exists {
		return nil
	}
	return known.readNow(store, name, read)
}

func (known KnownCollections) readNow(store CollectionStore, name string, read func() error) error {
	return known.afterRead(store, name, read())
}

func (known KnownCollections) afterRead(store CollectionStore, name string, failure error) error {
	if failure == nil {
		return nil
	}
	return known.readFailed(store, name, failure)
}

func (known KnownCollections) readFailed(store CollectionStore, name string, failure error) error {
	known.Forget(name)
	exists, err := known.asked(store, name)
	return known.failureUnless(known.confirmedMissing(exists, err), failure)
}

func (known KnownCollections) afterWrite(store CollectionStore, name string, write func() error, failure error) error {
	if failure == nil {
		return nil
	}
	return known.writeFailed(store, name, write, failure)
}

func (known KnownCollections) writeFailed(store CollectionStore, name string, write func() error, failure error) error {
	known.Forget(name)
	exists, err := known.asked(store, name)
	again := func() error { return known.retry(store, name, write) }
	return known.retryWhen(known.confirmedMissing(exists, err), failure, again)
}

func (KnownCollections) retryWhen(missing bool, failure error, again func() error) error {
	if !missing {
		return failure
	}
	return again()
}

func (known KnownCollections) retry(store CollectionStore, name string, write func() error) error {
	err := known.create(store, name)
	if err != nil {
		return err
	}
	return write()
}

func (KnownCollections) confirmedMissing(exists bool, err error) bool {
	if err != nil {
		return false
	}
	return !exists
}

func (KnownCollections) failureUnless(settled bool, failure error) error {
	if settled {
		return nil
	}
	return failure
}

func (known KnownCollections) markerPath(name string) string {
	if !regexp.MustCompile(known.namePattern()).MatchString(name) {
		return ""
	}
	return filepath.Join(known.folder(), name)
}

func (known KnownCollections) folder() string {
	return cmp.Or(known.Folder, known.containerFolder())
}

func (known KnownCollections) containerFolder() string {
	digest := sha256.Sum256([]byte(os.Getenv(known.settingsVariable())))
	return filepath.Join(os.TempDir(), known.folderName(), hex.EncodeToString(digest[:known.keyBytes()]))
}

func (KnownCollections) settingsVariable() string {
	return "YON_APPLICATION_JSON"
}

func (KnownCollections) folderName() string {
	return "mesail-known-collections"
}

func (KnownCollections) namePattern() string {
	return `^[a-z0-9][a-z0-9-]{0,127}$`
}

func (KnownCollections) keyBytes() int {
	return 8
}

func (KnownCollections) folderMode() os.FileMode {
	return 0o700
}

func (KnownCollections) markerMode() os.FileMode {
	return 0o600
}
