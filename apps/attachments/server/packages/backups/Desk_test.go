package backups

import (
	"errors"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

const ana = "Bearer ana@example.com"

func expectConflict(t *testing.T, err error, number int) {
	t.Helper()
	conflict := Conflict{}
	shared := attachments.Conflict{}
	if errors.As(err, &shared) && shared.ErrorNumber == number {
		return
	}
	if !errors.As(err, &conflict) || conflict.ErrorNumber != number {
		t.Fatalf("want conflict %d, got %v", number, err)
	}
}

func TestMineWithoutABackupIsMissing(t *testing.T) {
	_, err := newFixture(t).desk.Mine(ana)
	if !errors.Is(err, Missing{}) {
		t.Fatalf("want missing, got %v", err)
	}
}

func TestStartCompleteMineAndDownload(t *testing.T) {
	fixture := newFixture(t)
	upload, err := fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
	if err != nil || upload.Version != 1 || upload.ExpiresAt != "2026-11-14T12:30:00Z" {
		t.Fatalf("start: %+v %v", upload, err)
	}
	if !strings.Contains(upload.UploadUrl, "/example.com/backup-") || !strings.Contains(upload.UploadUrl, "application/octet-stream") {
		t.Fatalf("upload url %s", upload.UploadUrl)
	}
	fixture.store.put(upload.UploadUrl, "cipher")
	fixture.now = fixture.now.Add(time.Minute)
	backup, err := fixture.desk.Complete(ana, 1)
	if err != nil || backup.Version != 1 || backup.Size != 6 || backup.UpdatedByDeviceId != "phone" || backup.UpdatedAt != "2026-11-14T12:01:00Z" {
		t.Fatalf("complete: %+v %v", backup, err)
	}
	mine, err := fixture.desk.Mine(ana)
	if err != nil || mine.Version != 1 || mine.Sha256 != digest("cipher") || len(mine.KeySlots) != 1 {
		t.Fatalf("mine: %+v %v", mine, err)
	}
	download, err := fixture.desk.Download(ana)
	if err != nil || !strings.Contains(download.Url, "mesail-backup.bin") || download.ExpiresAt != "2026-11-14T12:31:00Z" {
		t.Fatalf("download: %+v %v", download, err)
	}
}

func TestANewVersionRemovesTheOldBytes(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "first")
	second := fixture.backedUp(t, ana, 1, "second")
	if second.Version != 2 || len(fixture.store.blobs) != 1 {
		t.Fatalf("want version 2 and one blob, got %+v and %d blobs", second, len(fixture.store.blobs))
	}
}

func TestStartWithAnOldVersionIsAConflict(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "first")
	_, err := fixture.desk.Start(ana, startFor(0, "other", "laptop"))
	expectConflict(t, err, 3519)
}

func TestCompleteAfterAnotherDeviceFinishedIsAConflict(t *testing.T) {
	fixture := newFixture(t)
	phone, _ := fixture.desk.Start(ana, startFor(0, "phone", "phone"))
	laptop, _ := fixture.desk.Start(ana, startFor(0, "laptop", "laptop"))
	fixture.store.put(laptop.UploadUrl, "laptop")
	fixture.store.put(phone.UploadUrl, "phone")
	if _, err := fixture.desk.Complete(ana, laptop.Version); err != nil {
		t.Fatalf("laptop complete: %v", err)
	}
	_, err := fixture.desk.Complete(ana, phone.Version)
	expectConflict(t, err, 3519)
}

func TestCompleteWithoutAStartIsAConflict(t *testing.T) {
	_, err := newFixture(t).desk.Complete(ana, 1)
	expectConflict(t, err, 3520)
}

func TestCompleteVersionZeroIsAConflict(t *testing.T) {
	_, err := newFixture(t).desk.Complete(ana, 0)
	expectConflict(t, err, 3519)
}

func TestCompleteAfterTheLinkLifeIsAConflict(t *testing.T) {
	fixture := newFixture(t)
	upload, _ := fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
	fixture.store.put(upload.UploadUrl, "cipher")
	fixture.now = fixture.now.Add(61 * time.Minute)
	_, err := fixture.desk.Complete(ana, 1)
	expectConflict(t, err, 3520)
}

func TestCompleteChecksTheBytes(t *testing.T) {
	cases := map[string]int{"": 3508, "cipheR": 3510, "ciph": 3509}
	for content, number := range cases {
		fixture := newFixture(t)
		upload, _ := fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
		if content != "" {
			fixture.store.put(upload.UploadUrl, content)
		}
		_, err := fixture.desk.Complete(ana, 1)
		expectConflict(t, err, number)
	}
}

func TestCompleteWithTooManyBytesIsTooLarge(t *testing.T) {
	fixture := newFixture(t)
	upload, _ := fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
	fixture.store.put(upload.UploadUrl, "cipher and more")
	_, err := fixture.desk.Complete(ana, 1)
	if !errors.As(err, &blobs.TooLarge{}) {
		t.Fatalf("want too large, got %v", err)
	}
}

func TestCompleteWhenTheStoreFailsIsUnavailable(t *testing.T) {
	fixture := newFixture(t)
	fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
	fixture.store.readFails = true
	_, err := fixture.desk.Complete(ana, 1)
	if !errors.As(err, &attachments.Unavailable{}) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

func TestARestartReplacesTheWaitingUpload(t *testing.T) {
	fixture := newFixture(t)
	first, _ := fixture.desk.Start(ana, startFor(0, "first", "phone"))
	fixture.store.put(first.UploadUrl, "first")
	second, _ := fixture.desk.Start(ana, startFor(0, "second", "phone"))
	if _, found := fixture.store.blobs[strings.TrimPrefix(strings.Split(first.UploadUrl, "?")[0], "https://blobs.test/")]; found {
		t.Fatal("the replaced upload kept its bytes")
	}
	fixture.store.put(second.UploadUrl, "second")
	backup, err := fixture.desk.Complete(ana, 1)
	if err != nil || backup.Sha256 != digest("second") {
		t.Fatalf("complete: %+v %v", backup, err)
	}
}

func TestAnotherPersonSeesNoBackup(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	_, mineErr := fixture.desk.Mine("Bearer bo@example.com")
	_, downloadErr := fixture.desk.Download("Bearer bo@example.com")
	_, otherDomainErr := fixture.desk.Mine("Bearer ana@example.org")
	if !errors.Is(mineErr, Missing{}) || !errors.Is(downloadErr, Missing{}) || !errors.Is(otherDomainErr, Missing{}) {
		t.Fatalf("want missing, got %v, %v, %v", mineErr, downloadErr, otherDomainErr)
	}
}

func TestDeleteRemovesTheRecordAndTheBytes(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	fixture.desk.Start(ana, startFor(1, "next", "phone"))
	upload, _ := fixture.desk.Start(ana, startFor(1, "next", "phone"))
	fixture.store.put(upload.UploadUrl, "next")
	if err := fixture.desk.Delete(ana); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := fixture.desk.Delete(ana); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	_, err := fixture.desk.Mine(ana)
	if !errors.Is(err, Missing{}) || len(fixture.store.blobs) != 0 {
		t.Fatalf("want nothing left, got %v and %d blobs", err, len(fixture.store.blobs))
	}
	again := fixture.backedUp(t, ana, 0, "again")
	if again.Version != 1 {
		t.Fatalf("a new backup after delete starts at 1, got %d", again.Version)
	}
}

func TestDeleteWhenBytesCannotBeRemovedKeepsTheRecord(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	fixture.store.deleteFail = true
	err := fixture.desk.Delete(ana)
	_, mineErr := fixture.desk.Mine(ana)
	if !errors.As(err, &attachments.Unavailable{}) || mineErr != nil {
		t.Fatalf("want unavailable and the record kept, got %v, %v", err, mineErr)
	}
}

func TestARefusedTokenIsUnauthenticated(t *testing.T) {
	desk := newFixture(t).desk
	_, mineErr := desk.Mine("Bearer nobody")
	_, startErr := desk.Start("Bearer nobody", startFor(0, "x", "phone"))
	_, completeErr := desk.Complete("Bearer nobody", 1)
	_, downloadErr := desk.Download("Bearer nobody")
	deleteErr := desk.Delete("Bearer nobody")
	for _, err := range []error{mineErr, startErr, completeErr, downloadErr, deleteErr} {
		if !errors.Is(err, attachments.Unauthenticated{}) {
			t.Fatalf("want unauthenticated, got %v", err)
		}
	}
}

func TestASetupFailureIsReturned(t *testing.T) {
	fixture := newFixture(t)
	fixture.desk.Failure = attachments.Unavailable{Operation: "setup"}
	_, err := fixture.desk.Start(ana, startFor(0, "x", "phone"))
	removeErr := fixture.desk.Remove("example.com", "ana")
	if !errors.As(err, &attachments.Unavailable{}) || !errors.As(removeErr, &attachments.Unavailable{}) {
		t.Fatalf("want unavailable, got %v, %v", err, removeErr)
	}
}

func TestStartRefusesABadRequestBeforeStoring(t *testing.T) {
	fixture := newFixture(t)
	request := startFor(0, "x", "phone")
	request.DeviceId = ""
	_, err := fixture.desk.Start(ana, request)
	if !errors.As(err, &Invalid{}) || len(fixture.documents.collections) != 0 {
		t.Fatalf("want invalid and nothing stored, got %v", err)
	}
}

func TestSigningFailuresAreUnavailable(t *testing.T) {
	fixture := newFixture(t)
	fixture.backedUp(t, ana, 0, "cipher")
	fixture.store.failOn = "upload"
	_, startErr := fixture.desk.Start(ana, startFor(1, "x", "phone"))
	fixture.store.failOn = "download"
	_, downloadErr := fixture.desk.Download(ana)
	if !errors.As(startErr, &attachments.Unavailable{}) || !errors.As(downloadErr, &attachments.Unavailable{}) {
		t.Fatalf("want unavailable, got %v, %v", startErr, downloadErr)
	}
}

func TestStorageFailuresAreUnavailable(t *testing.T) {
	for _, operation := range []string{"scroll", "insert", "patch", "delete"} {
		fixture := newFixture(t)
		fixture.backedUp(t, ana, 0, "cipher")
		fixture.documents.failOn = operation
		_, startErr := fixture.desk.Start(ana, startFor(1, "x", "phone"))
		_, completeErr := fixture.desk.Complete(ana, 2)
		deleteErr := fixture.desk.Delete(ana)
		failed := errors.As(startErr, &attachments.Unavailable{}) || errors.As(completeErr, &attachments.Unavailable{}) || errors.As(deleteErr, &attachments.Unavailable{})
		if operation != "insert" && !failed {
			t.Fatalf("%s: want a failure, got %v, %v, %v", operation, startErr, completeErr, deleteErr)
		}
	}
}

func TestCompleteSaveFailureIsUnavailable(t *testing.T) {
	fixture := newFixture(t)
	upload, _ := fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
	fixture.store.put(upload.UploadUrl, "cipher")
	fixture.documents.failOn = "patch"
	_, err := fixture.desk.Complete(ana, 1)
	if !errors.As(err, &attachments.Unavailable{}) {
		t.Fatalf("want unavailable, got %v", err)
	}
}

func TestStartSaveFailureIsUnavailable(t *testing.T) {
	fixture := newFixture(t)
	fixture.documents.failOn = "insert"
	_, err := fixture.desk.Start(ana, startFor(0, "cipher", "phone"))
	if !errors.As(err, &attachments.Unavailable{}) {
		t.Fatalf("want unavailable, got %v", err)
	}
}
