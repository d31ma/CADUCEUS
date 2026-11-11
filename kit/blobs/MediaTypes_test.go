package blobs

import "testing"

func TestValidAcceptsTypeAndSubtype(t *testing.T) {
	for _, mediaType := range []string{"image/png", "text/plain; charset=utf-8", "application/vnd.openxmlformats-officedocument.wordprocessingml.document"} {
		if !(MediaTypes{}).Valid(mediaType) {
			t.Fatalf("%q refused", mediaType)
		}
	}
}

func TestValidRefusesMalformedTypes(t *testing.T) {
	for _, mediaType := range []string{"", "png", "image/png/x", "image/png; =", "text/html\r\nX: y"} {
		if (MediaTypes{}).Valid(mediaType) {
			t.Fatalf("%q accepted", mediaType)
		}
	}
}

func TestConsistentAcceptsMatchingAndVagueSniffs(t *testing.T) {
	pairs := [][2]string{
		{"image/png", "image/png"}, {"application/zip", "application/octet-stream"}, {"text/csv", "text/plain; charset=utf-8"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "application/zip"}, {"image/svg+xml", "text/xml; charset=utf-8"},
		{"text/html; charset=utf-8", "text/html; charset=utf-8"},
	}
	for _, pair := range pairs {
		if !(MediaTypes{}).Consistent(pair[0], pair[1]) {
			t.Fatalf("%q with sniffed %q refused", pair[0], pair[1])
		}
	}
}

func TestConsistentBlocksDisguisedContent(t *testing.T) {
	pairs := [][2]string{{"image/png", "image/jpeg"}, {"image/png", "application/pdf"}, {"application/pdf", "text/html; charset=utf-8"}, {"image/gif", "text/html; charset=utf-8"}}
	for _, pair := range pairs {
		if (MediaTypes{}).Consistent(pair[0], pair[1]) {
			t.Fatalf("%q with sniffed %q accepted", pair[0], pair[1])
		}
	}
}

func TestAttachmentDispositionQuotesAsciiNames(t *testing.T) {
	if got := (Disposition{}).Attachment("Q3 report.pdf"); got != `attachment; filename="Q3 report.pdf"` {
		t.Fatalf("got %q", got)
	}
}

func TestAttachmentDispositionEncodesOtherNames(t *testing.T) {
	if got := (Disposition{}).Attachment("résumé.pdf"); got != `attachment; filename*=utf-8''r%C3%A9sum%C3%A9.pdf` {
		t.Fatalf("got %q", got)
	}
}

func TestAttachmentDispositionWithoutANameIsPlain(t *testing.T) {
	if got := (Disposition{}).Attachment(""); got != `attachment; filename=""` {
		t.Fatalf("got %q", got)
	}
}
