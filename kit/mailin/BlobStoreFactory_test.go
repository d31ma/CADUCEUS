package mailin

import (
	"strings"
	"testing"

	"mesail.dev/packages/blobs"
)

func TestALocalStoreNeedsAFolderAndASecret(t *testing.T) {
	store := BlobStoreFactory{}.Store(BlobSettings{Adapter: "local", Folder: t.TempDir()})
	_, isMissing := store.(MissingBlobs)
	check(t, isMissing, "want missing blobs")
}

func TestALocalStoreWritesIntoItsFolder(t *testing.T) {
	settings := BlobSettings{Adapter: "local", Folder: t.TempDir(), Link: LinkSettings{Base: "https://mesail.test/blobs", Secret: "fixture-link-value"}}
	store := BlobStoreFactory{}.Store(settings)
	key := blobs.Key{DomainId: "example.com", Name: "inbound-1"}
	err := store.Write(key, strings.NewReader("raw"), 3)
	url, signErr := store.SignDownload(key, MessageDesk{}.OriginalTerms(), fixedNow())
	check(t, err == nil && signErr == nil, "local store failed")
	check(t, strings.HasPrefix(url, "https://mesail.test/blobs/example.com/inbound-1?"), "wrong link")
}

func TestTheRemovedS3AdapterRefusesEveryCall(t *testing.T) {
	settings := BlobSettings{Adapter: "s3", Folder: t.TempDir(), Link: LinkSettings{Secret: "fixture-link-value"}}
	store := BlobStoreFactory{}.Store(settings)
	err := store.Write(blobs.Key{DomainId: "example.com", Name: "inbound-1"}, strings.NewReader("raw"), 3)
	check(t, isUnavailable(err, `the mail-in setting blobs.adapter "s3" was removed with the cloud option (ADR 0007): keep blobs in a local folder with the adapter "local"`), "want the removed adapter named")
}

func TestAnS3BucketRefusesEveryCall(t *testing.T) {
	settings := BlobSettings{Adapter: "local", Bucket: "mesail-raw", Folder: t.TempDir(), Link: LinkSettings{Secret: "fixture-link-value"}}
	_, err := BlobStoreFactory{}.Store(settings).Read(blobs.Key{DomainId: "example.com", Name: "inbound-1"})
	check(t, err != nil && strings.Contains(err.Error(), `blobs.bucket "mesail-raw" was removed`), "want the removed bucket named")
}

func TestMissingBlobsRefuseEveryCall(t *testing.T) {
	missing, key := MissingBlobs{}, blobs.Key{DomainId: "example.com", Name: "inbound-1"}
	_, upload := missing.SignUpload(key, blobs.UploadTerms{}, fixedNow())
	_, download := missing.SignDownload(key, blobs.DownloadTerms{}, fixedNow())
	_, read := missing.Read(key)
	check(t, isUnavailable(upload, "blob store settings") && isUnavailable(download, "blob store settings"), "want unavailable")
	check(t, isUnavailable(read, "blob store settings"), "want unavailable")
	check(t, isUnavailable(missing.Write(key, strings.NewReader(""), 0), "blob store settings"), "want unavailable")
	check(t, isUnavailable(missing.Copy(key, key), "blob store settings"), "want unavailable")
	check(t, isUnavailable(missing.Delete(key), "blob store settings"), "want unavailable")
}
