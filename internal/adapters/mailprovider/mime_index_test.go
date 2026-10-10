package mailprovider

import (
	"strings"
	"testing"
)

func TestIndexRFC822ExtractsBoundedSearchAndAttachmentMetadata(t *testing.T) {
	raw := []byte("From: =?UTF-8?Q?Jos=C3=A9?= <sender@example.net>\r\n" +
		"To: Alice <alice@example.com>, ALICE@example.com\r\nCc: team@example.net\r\n" +
		"Subject: =?UTF-8?Q?Quarterly_caf=C3=A9?=\r\n" +
		"Content-Type: multipart/mixed; boundary=outer\r\n\r\n" +
		"--outer\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nIndexed provider body\r\n" +
		"--outer\r\nContent-Type: application/pdf; name=report.pdf\r\nContent-Disposition: attachment; filename=report.pdf\r\n\r\nPDF\r\n--outer--\r\n")
	index := indexRFC822(raw)
	if index.Subject != "Quarterly café" || index.Sender != "José <sender@example.net>" || index.Snippet != "Indexed provider body" {
		t.Fatalf("index = %#v", index)
	}
	if len(index.To) != 1 || index.To[0] != "alice@example.com" || len(index.CC) != 1 || len(index.AttachmentNames) != 1 || index.AttachmentNames[0] != "report.pdf" {
		t.Fatalf("addresses/attachments = %#v", index)
	}
}

func TestIndexRFC822FallsBackToSanitizedHTMLAndPortableFilename(t *testing.T) {
	raw := []byte("From: sender@example.net\r\nTo: alice@example.com\r\nSubject: HTML\r\n" +
		"Content-Type: multipart/mixed; boundary=x\r\n\r\n--x\r\nContent-Type: text/html\r\n\r\n<p>Hello &amp; goodbye</p>\r\n" +
		"--x\r\nContent-Disposition: attachment; filename=\"../../secret.txt\"\r\n\r\nx\r\n--x--\r\n")
	index := indexRFC822(raw)
	if index.Snippet != "Hello & goodbye" || len(index.AttachmentNames) != 1 || index.AttachmentNames[0] != "secret.txt" {
		t.Fatalf("index = %#v", index)
	}
}

func TestIndexRFC822FindsAttachmentsAfterTextBudgetIsExhausted(t *testing.T) {
	raw := []byte("Content-Type: multipart/mixed; boundary=fixture\r\n\r\n" +
		"--fixture\r\nContent-Type: text/plain\r\n\r\n" + strings.Repeat("x", (1<<20)+4096) + "\r\n" +
		"--fixture\r\nContent-Type: application/pdf; name=report.pdf\r\n" +
		"Content-Disposition: attachment; filename=report.pdf\r\n\r\nbytes\r\n" +
		"--fixture--\r\n")
	index := indexRFC822(raw)
	if len(index.Snippet) != 4096 {
		t.Fatalf("snippet length = %d, want 4096", len(index.Snippet))
	}
	if len(index.AttachmentNames) != 1 || index.AttachmentNames[0] != "report.pdf" {
		t.Fatalf("attachment names = %#v", index.AttachmentNames)
	}
}
