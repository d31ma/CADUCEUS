package attachments

import (
	"errors"
	"strings"
	"testing"

	"mesail.dev/packages/blobs"
)

func TestCreateStoresAPendingRecordAndSignsAnUploadLink(t *testing.T) {
	desk := newTestDesk(t)
	upload, err := desk.desk.Create(token("example.com", "ana"), request("hello", "text/plain"))
	if err != nil || !strings.HasPrefix(upload.AttachmentId, "att_") || !strings.HasPrefix(upload.UploadUrl, "http://127.0.0.1:8100/blobs/example.com/") {
		t.Fatalf("got %+v, %v", upload, err)
	}
	record, _ := desk.desk.Book.Lookup("example.com", upload.AttachmentId)
	if record.State != "pending" || record.UploaderId != "ana" || record.CreatedAt != "2026-11-10T12:00:00Z" || upload.ExpiresAt != "2026-11-10T12:05:00Z" {
		t.Fatalf("got %+v and %+v", record, upload)
	}
}

func TestCreateRefusesAForgedToken(t *testing.T) {
	desk := newTestDesk(t)
	if _, err := desk.desk.Create("Bearer forged", request("hello", "text/plain")); !errors.Is(err, Unauthenticated{}) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateReportsAStoreThatIsNotConfigured(t *testing.T) {
	desk := newTestDesk(t)
	desk.desk.Failure = Unavailable{Operation: "setup"}
	if _, err := desk.desk.Create(token("example.com", "ana"), request("hello", "text/plain")); !errors.Is(err, Unavailable{Operation: "setup"}) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRefusesAFileOverTheLimit(t *testing.T) {
	desk := newTestDesk(t)
	var large blobs.TooLarge
	if _, err := desk.desk.Create(token("example.com", "ana"), request(strings.Repeat("x", 101), "text/plain")); !errors.As(err, &large) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateReportsAStorageFailure(t *testing.T) {
	desk := newTestDesk(t)
	desk.memory.failOn = "insert"
	if _, err := desk.desk.Create(token("example.com", "ana"), request("hello", "text/plain")); !errors.Is(err, Unavailable{Operation: "insert"}) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateReportsASigningFailure(t *testing.T) {
	desk := newTestDesk(t)
	if _, err := desk.desk.Create(token("Bad Domain", "ana"), request("hello", "text/plain")); !errors.Is(err, Unavailable{Operation: "sign upload"}) {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteMarksAMatchingUploadReady(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	view, err := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	if err != nil || view.State != "ready" || view.AttachmentId != upload.AttachmentId || view.Size != 5 || view.FileName != "plan.txt" {
		t.Fatalf("got %+v, %v", view, err)
	}
}

func TestCompleteTwiceAnswersTheSameView(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	first, _ := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	second, err := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	if err != nil || first != second {
		t.Fatalf("got %+v and %+v, %v", first, second, err)
	}
}

func TestCompleteBlocksDisguisedContent(t *testing.T) {
	desk := newTestDesk(t)
	content := "<!DOCTYPE html><html><script>alert(1)</script></html>"
	upload, _ := desk.desk.Create(token("example.com", "ana"), request(content, "image/png"))
	record, _ := desk.desk.Book.Lookup("example.com", upload.AttachmentId)
	desk.folder.Write(desk.desk.Book.Key(record), strings.NewReader(content), int64(len(content)))
	view, err := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	if err != nil || view.State != "blocked" {
		t.Fatalf("got %+v, %v", view, err)
	}
	if _, err := desk.desk.Download(token("example.com", "ana"), upload.AttachmentId); !errors.As(err, &Conflict{}) {
		t.Fatalf("a blocked file was offered: %v", err)
	}
}

func TestCompleteRefusesTheWrongBytes(t *testing.T) {
	cases := map[string]string{"jello": "sha256", "hell": "size", "": "file"}
	for stored, field := range cases {
		desk := newTestDesk(t)
		upload, _ := desk.desk.Create(token("example.com", "ana"), request("hello", "text/plain"))
		record, _ := desk.desk.Book.Lookup("example.com", upload.AttachmentId)
		if stored != "" {
			desk.folder.Write(desk.desk.Book.Key(record), strings.NewReader(stored), int64(len(stored)))
		}
		var conflict Conflict
		if _, err := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId); !errors.As(err, &conflict) || conflict.Field != field {
			t.Fatalf("%q: got %v", stored, err)
		}
	}
}

func TestCompleteRefusesALargerFile(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello world")
	var large blobs.TooLarge
	if _, err := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId); !errors.As(err, &large) {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteByAnotherMemberOrDomainIsMissing(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	for _, authorization := range []string{token("example.com", "bo"), token("example.org", "ana"), "Bearer forged"} {
		if _, err := desk.desk.Complete(authorization, upload.AttachmentId); err == nil {
			t.Fatalf("%s completed the upload", authorization)
		}
	}
	if _, err := desk.desk.Complete(token("example.com", "bo"), upload.AttachmentId); !errors.Is(err, Missing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteReportsAStorageFailure(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	desk.memory.failOn = "patch"
	if _, err := desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId); !errors.Is(err, Unavailable{Operation: "state"}) {
		t.Fatalf("got %v", err)
	}
}

func TestDownloadSignsALinkForTheUploader(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	download, err := desk.desk.Download(token("example.com", "ana"), upload.AttachmentId)
	if err != nil || !strings.Contains(download.Url, "/blobs/example.com/") || !strings.Contains(download.Url, "file=plan.txt") || download.ExpiresAt != "2026-11-10T12:05:00Z" {
		t.Fatalf("got %+v, %v", download, err)
	}
	if desk.readers.asked != 0 {
		t.Fatal("messaging was asked about the uploader")
	}
}

func TestDownloadAsksMessagingAboutOtherMembers(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	if _, err := desk.desk.Download(token("example.com", "bo"), upload.AttachmentId); !errors.Is(err, Missing{}) {
		t.Fatalf("bo without access: %v", err)
	}
	desk.readers.allowed = true
	if _, err := desk.desk.Download(token("example.com", "bo"), upload.AttachmentId); err != nil {
		t.Fatalf("bo with access: %v", err)
	}
}

func TestDownloadNeverCrossesADomain(t *testing.T) {
	desk := newTestDesk(t)
	desk.readers.allowed = true
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	if _, err := desk.desk.Download(token("example.org", "ana"), upload.AttachmentId); !errors.Is(err, Missing{}) {
		t.Fatalf("got %v", err)
	}
}

func TestDownloadAllowsAServiceOfTheDomain(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	desk.desk.Complete(token("example.com", "ana"), upload.AttachmentId)
	if _, err := desk.desk.Download("Bearer example.com|mail-out|Service", upload.AttachmentId); err != nil {
		t.Fatalf("got %v", err)
	}
}

func TestDownloadOfAPendingUploadIsAConflict(t *testing.T) {
	desk := newTestDesk(t)
	upload := desk.uploaded(t, token("example.com", "ana"), "hello", "hello")
	var conflict Conflict
	if _, err := desk.desk.Download(token("example.com", "ana"), upload.AttachmentId); !errors.As(err, &conflict) || conflict.Field != "state" {
		t.Fatalf("got %v", err)
	}
}

func TestDownloadRefusesAForgedTokenAndAMissingAttachment(t *testing.T) {
	desk := newTestDesk(t)
	_, forged := desk.desk.Download("Bearer forged", "att_D1")
	_, missing := desk.desk.Download(token("example.com", "ana"), "att_D9")
	if !errors.Is(forged, Unauthenticated{}) || !errors.Is(missing, Missing{}) {
		t.Fatalf("got %v and %v", forged, missing)
	}
}

func TestDownloadReportsASigningFailure(t *testing.T) {
	desk := newTestDesk(t)
	record, _ := desk.desk.Book.Insert(Record{DomainId: "example.com", UploaderId: "ana", State: "ready", BlobKey: "example.com/../x"})
	if _, err := desk.desk.Download(token("example.com", "ana"), desk.desk.Book.WireId(record)); !errors.Is(err, Unavailable{Operation: "sign download"}) {
		t.Fatalf("got %v", err)
	}
}
