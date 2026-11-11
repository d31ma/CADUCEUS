package compose_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/compose"
)

type countingBoundaries struct {
	count *int
}

func (source countingBoundaries) Next() string {
	*source.count = *source.count + 1
	return fmt.Sprintf("fixed-boundary-%d", *source.count)
}

func fixedComposer() compose.Composer {
	count := 0
	return compose.Composer{Boundaries: countingBoundaries{count: &count}}
}

func baseMessage() compose.Message {
	return compose.Message{
		From:      compose.Address{Name: "Ada Lovelace", Email: "ada@example.com"},
		To:        []compose.Address{{Name: "Bob", Email: "bob@example.org"}},
		Subject:   "Hello there",
		MessageId: "abc123@example.com",
		Date:      time.Date(2026, 11, 10, 9, 30, 0, 0, time.UTC),
		TextBody:  "Hello Bob.",
	}
}

func composeOrFail(t *testing.T, message compose.Message) []byte {
	t.Helper()
	raw, err := fixedComposer().Compose(message)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}
	return raw
}

func parse(t *testing.T, raw []byte) *mail.Message {
	t.Helper()
	parsed, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("output is not a valid message: %v\n%s", err, raw)
	}
	return parsed
}

func readAll(t *testing.T, reader io.Reader) string {
	t.Helper()
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	return string(content)
}

func multipartReader(t *testing.T, header mail.Header, body io.Reader, wantType string) *multipart.Reader {
	t.Helper()
	return partsOf(t, header.Get("Content-Type"), body, wantType)
}

func partsOf(t *testing.T, contentType string, body io.Reader, wantType string) *multipart.Reader {
	t.Helper()
	mediaType, parameters, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("bad Content-Type %q: %v", contentType, err)
	}
	if mediaType != wantType {
		t.Fatalf("Content-Type = %q, want %q", mediaType, wantType)
	}
	return multipart.NewReader(body, parameters["boundary"])
}

func nextPart(t *testing.T, reader *multipart.Reader) *multipart.Part {
	t.Helper()
	part, err := reader.NextRawPart()
	if err != nil {
		t.Fatalf("missing part: %v", err)
	}
	return part
}

func assertNoMorePart(t *testing.T, reader *multipart.Reader) {
	t.Helper()
	_, err := reader.NextRawPart()
	if err != io.EOF {
		t.Fatalf("expected no more parts, got %v", err)
	}
}

func headerNames(raw []byte) []string {
	head := strings.SplitN(string(raw), "\r\n\r\n", 2)[0]
	names := []string{}
	for _, line := range strings.Split(head, "\r\n") {
		if strings.HasPrefix(line, " ") {
			continue
		}
		names = append(names, strings.SplitN(line, ":", 2)[0])
	}
	return names
}

func assertField(t *testing.T, err error, want string) {
	t.Helper()
	var unsafe compose.UnsafeHeaderValue
	if !errors.As(err, &unsafe) {
		t.Fatalf("error = %v, want UnsafeHeaderValue", err)
	}
	if unsafe.Field != want {
		t.Fatalf("Field = %q, want %q", unsafe.Field, want)
	}
	if unsafe.Error() == "" {
		t.Fatal("empty error text")
	}
}

func assertMissing(t *testing.T, err error, want string) {
	t.Helper()
	var missing compose.MissingField
	if !errors.As(err, &missing) {
		t.Fatalf("error = %v, want MissingField", err)
	}
	if missing.Field != want {
		t.Fatalf("Field = %q, want %q", missing.Field, want)
	}
	if missing.Error() == "" {
		t.Fatal("empty error text")
	}
}

func assertLineEnds(t *testing.T, raw []byte) {
	t.Helper()
	text := string(raw)
	if strings.Count(text, "\n") != strings.Count(text, "\r\n") {
		t.Fatal("found a line feed without a carriage return")
	}
	if strings.Count(text, "\r") != strings.Count(text, "\r\n") {
		t.Fatal("found a carriage return without a line feed")
	}
	if !strings.HasSuffix(text, "\r\n") {
		t.Fatal("message does not end with CRLF")
	}
	for _, line := range strings.Split(text, "\r\n") {
		if len(line) > 998 {
			t.Fatalf("line longer than 998 characters: %d", len(line))
		}
	}
}

func assertHeaderLinesWithin(t *testing.T, raw []byte, width int) {
	t.Helper()
	head := strings.SplitN(string(raw), "\r\n\r\n", 2)[0]
	for _, line := range strings.Split(head, "\r\n") {
		if len(line) > width {
			t.Fatalf("header line longer than %d: %q", width, line)
		}
	}
}

func TestTextOnlyMessageHasPlainQuotedPrintableBody(t *testing.T) {
	parsed := parse(t, composeOrFail(t, baseMessage()))
	if got := parsed.Header.Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := parsed.Header.Get("Content-Transfer-Encoding"); got != "quoted-printable" {
		t.Fatalf("Content-Transfer-Encoding = %q", got)
	}
	body := readAll(t, quotedprintable.NewReader(parsed.Body))
	if strings.TrimRight(body, "\r\n") != "Hello Bob." {
		t.Fatalf("body = %q", body)
	}
}

func TestBasicHeadersAreWritten(t *testing.T) {
	parsed := parse(t, composeOrFail(t, baseMessage()))
	from, err := mail.ParseAddress(parsed.Header.Get("From"))
	if err != nil || from.Name != "Ada Lovelace" || from.Address != "ada@example.com" {
		t.Fatalf("From = %v (%v)", from, err)
	}
	to, err := parsed.Header.AddressList("To")
	if err != nil || len(to) != 1 || to[0].Address != "bob@example.org" {
		t.Fatalf("To = %v (%v)", to, err)
	}
	if got := parsed.Header.Get("Subject"); got != "Hello there" {
		t.Fatalf("Subject = %q", got)
	}
	if got := parsed.Header.Get("Message-ID"); got != "<abc123@example.com>" {
		t.Fatalf("Message-ID = %q", got)
	}
	if got := parsed.Header.Get("MIME-Version"); got != "1.0" {
		t.Fatalf("MIME-Version = %q", got)
	}
	date, err := parsed.Header.Date()
	if err != nil || !date.Equal(baseMessage().Date) {
		t.Fatalf("Date = %v (%v)", date, err)
	}
}

func TestHeadersAreInTheAgreedOrder(t *testing.T) {
	message := baseMessage()
	message.Cc = []compose.Address{{Email: "carol@example.org"}}
	message.ReplyTo = []compose.Address{{Email: "reply@example.com"}}
	message.InReplyTo = "parent@example.com"
	message.References = []string{"root@example.com", "parent@example.com"}
	got := strings.Join(headerNames(composeOrFail(t, message)), ",")
	want := "From,To,Cc,Reply-To,Subject,Date,Message-ID,In-Reply-To,References,MIME-Version,Content-Type,Content-Transfer-Encoding"
	if got != want {
		t.Fatalf("header order = %s\nwant %s", got, want)
	}
}

func TestReplyToHeaderListsAddresses(t *testing.T) {
	message := baseMessage()
	message.ReplyTo = []compose.Address{{Email: "reply@example.com"}, {Name: "Desk", Email: "desk@example.com"}}
	parsed := parse(t, composeOrFail(t, message))
	list, err := parsed.Header.AddressList("Reply-To")
	if err != nil || len(list) != 2 || list[1].Name != "Desk" {
		t.Fatalf("Reply-To = %v (%v)", list, err)
	}
}

func TestThreadingHeadersUseAngleBrackets(t *testing.T) {
	message := baseMessage()
	message.InReplyTo = "parent@example.com"
	message.References = []string{"root@example.com", "<parent@example.com>"}
	parsed := parse(t, composeOrFail(t, message))
	if got := parsed.Header.Get("In-Reply-To"); got != "<parent@example.com>" {
		t.Fatalf("In-Reply-To = %q", got)
	}
	if got := parsed.Header.Get("References"); got != "<root@example.com> <parent@example.com>" {
		t.Fatalf("References = %q", got)
	}
}

func TestReferencesAreCappedKeepingTheFirst(t *testing.T) {
	message := baseMessage()
	message.References = []string{}
	for index := 0; index < 30; index++ {
		message.References = append(message.References, fmt.Sprintf("reference-%02d@example.com", index))
	}
	raw := composeOrFail(t, message)
	got := strings.Fields(parse(t, raw).Header.Get("References"))
	if len(got) != 20 {
		t.Fatalf("kept %d references, want 20", len(got))
	}
	if got[0] != "<reference-00@example.com>" || got[1] != "<reference-11@example.com>" || got[19] != "<reference-29@example.com>" {
		t.Fatalf("References = %v", got)
	}
	assertHeaderLinesWithin(t, raw, 78)
}

func TestMissingOptionalHeadersAreLeftOut(t *testing.T) {
	message := baseMessage()
	message.MessageId = ""
	names := strings.Join(headerNames(composeOrFail(t, message)), ",")
	if strings.Contains(names, "Message-ID") || strings.Contains(names, "In-Reply-To") || strings.Contains(names, "References") || strings.Contains(names, "Cc") {
		t.Fatalf("unexpected headers: %s", names)
	}
}

func TestZeroDateUsesTheCurrentTime(t *testing.T) {
	message := baseMessage()
	message.Date = time.Time{}
	date, err := parse(t, composeOrFail(t, message)).Header.Date()
	if err != nil || time.Since(date) > time.Minute {
		t.Fatalf("Date = %v (%v)", date, err)
	}
}

func TestNoBccHeaderIsEverWritten(t *testing.T) {
	message := baseMessage()
	message.Cc = []compose.Address{{Email: "carol@example.org"}}
	message.HtmlBody = "<p>Hello</p>"
	raw := composeOrFail(t, message)
	if strings.Contains(strings.ToLower(string(raw)), "bcc") {
		t.Fatalf("output mentions Bcc:\n%s", raw)
	}
}

func TestNonAsciiSubjectAndNamesUseEncodedWords(t *testing.T) {
	message := baseMessage()
	message.Subject = "Grüße aus Köln"
	message.From.Name = "Jürgen Müller"
	raw := composeOrFail(t, message)
	parsed := parse(t, raw)
	if !strings.Contains(string(raw), "=?utf-8?") {
		t.Fatalf("no encoded word in:\n%s", raw)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || subject != message.Subject {
		t.Fatalf("Subject = %q (%v)", subject, err)
	}
	from, err := mail.ParseAddress(parsed.Header.Get("From"))
	if err != nil || from.Name != message.From.Name {
		t.Fatalf("From = %v (%v)", from, err)
	}
	assertLineEnds(t, raw)
}

func TestLongNonAsciiSubjectIsFolded(t *testing.T) {
	message := baseMessage()
	message.Subject = strings.Repeat("été ", 60)
	raw := composeOrFail(t, message)
	subject, err := new(mime.WordDecoder).DecodeHeader(parse(t, raw).Header.Get("Subject"))
	if err != nil || subject != message.Subject {
		t.Fatalf("Subject = %q (%v)", subject, err)
	}
	assertHeaderLinesWithin(t, raw, 78)
}

func TestLongAsciiSubjectIsFolded(t *testing.T) {
	message := baseMessage()
	message.Subject = strings.TrimSpace(strings.Repeat("word ", 300))
	raw := composeOrFail(t, message)
	if got := parse(t, raw).Header.Get("Subject"); got != message.Subject {
		t.Fatalf("Subject = %q", got)
	}
	assertHeaderLinesWithin(t, raw, 78)
	assertLineEnds(t, raw)
}

func TestLongTextBodyKeepsLinesShort(t *testing.T) {
	message := baseMessage()
	message.TextBody = strings.Repeat("x", 3000) + "\nsecond line\r\nthird line\n"
	raw := composeOrFail(t, message)
	assertLineEnds(t, raw)
	body := readAll(t, quotedprintable.NewReader(parse(t, raw).Body))
	if body != strings.Repeat("x", 3000)+"\r\nsecond line\r\nthird line\r\n" {
		t.Fatalf("body round trip failed: %q", body[len(body)-40:])
	}
}

func TestTextAndHtmlMakeMultipartAlternative(t *testing.T) {
	message := baseMessage()
	message.HtmlBody = "<p>Hello <b>Bob</b>.</p>"
	raw := composeOrFail(t, message)
	assertLineEnds(t, raw)
	parsed := parse(t, raw)
	reader := multipartReader(t, parsed.Header, parsed.Body, "multipart/alternative")
	plain := nextPart(t, reader)
	if plain.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("first part = %q", plain.Header.Get("Content-Type"))
	}
	if got := readAll(t, quotedprintable.NewReader(plain)); got != "Hello Bob." {
		t.Fatalf("text = %q", got)
	}
	html := nextPart(t, reader)
	if html.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("second part = %q", html.Header.Get("Content-Type"))
	}
	if got := readAll(t, quotedprintable.NewReader(html)); got != message.HtmlBody {
		t.Fatalf("html = %q", got)
	}
	assertNoMorePart(t, reader)
}

func TestAttachmentsMakeMultipartMixed(t *testing.T) {
	content := bytes.Repeat([]byte("0123456789abcdef"), 40)
	message := baseMessage()
	message.HtmlBody = "<p>Hi</p>"
	message.Attachments = []compose.Attachment{
		{FileName: "report.pdf", ContentType: "application/pdf", Content: content},
		{FileName: "Résumé 2026.txt", Content: []byte("plain")},
	}
	raw := composeOrFail(t, message)
	assertLineEnds(t, raw)
	parsed := parse(t, raw)
	mixed := multipartReader(t, parsed.Header, parsed.Body, "multipart/mixed")
	body := nextPart(t, mixed)
	alternative := partsOf(t, body.Header.Get("Content-Type"), body, "multipart/alternative")
	nextPart(t, alternative)
	nextPart(t, alternative)
	assertNoMorePart(t, alternative)

	first := nextPart(t, mixed)
	if first.FileName() != "report.pdf" || first.Header.Get("Content-Type") != "application/pdf" {
		t.Fatalf("first attachment = %q %q", first.FileName(), first.Header.Get("Content-Type"))
	}
	if !strings.HasPrefix(first.Header.Get("Content-Disposition"), "attachment;") {
		t.Fatalf("Content-Disposition = %q", first.Header.Get("Content-Disposition"))
	}
	encoded := readAll(t, first)
	for _, line := range strings.Split(strings.TrimRight(encoded, "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("base64 line longer than 76: %d", len(line))
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\r\n", ""))
	if err != nil || !bytes.Equal(decoded, content) {
		t.Fatalf("attachment content changed (%v)", err)
	}

	second := nextPart(t, mixed)
	if second.FileName() != "Résumé 2026.txt" {
		t.Fatalf("second file name = %q", second.FileName())
	}
	if !strings.Contains(second.Header.Get("Content-Disposition"), "filename*=utf-8''") {
		t.Fatalf("file name not RFC 2231 encoded: %q", second.Header.Get("Content-Disposition"))
	}
	if second.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("default content type = %q", second.Header.Get("Content-Type"))
	}
	assertNoMorePart(t, mixed)
}

func TestTextOnlyWithAttachmentWrapsThePlainPart(t *testing.T) {
	message := baseMessage()
	message.Attachments = []compose.Attachment{{FileName: "empty.bin", ContentType: "application/octet-stream"}}
	parsed := parse(t, composeOrFail(t, message))
	mixed := multipartReader(t, parsed.Header, parsed.Body, "multipart/mixed")
	body := nextPart(t, mixed)
	if body.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatalf("body part = %q", body.Header.Get("Content-Type"))
	}
	attachment := nextPart(t, mixed)
	if got := readAll(t, attachment); strings.TrimSpace(got) != "" {
		t.Fatalf("empty attachment body = %q", got)
	}
	assertNoMorePart(t, mixed)
}

func TestGivenBoundarySourceMakesOutputDeterministic(t *testing.T) {
	message := baseMessage()
	message.HtmlBody = "<p>Hi</p>"
	message.Attachments = []compose.Attachment{{FileName: "a.txt", ContentType: "text/plain", Content: []byte("a")}}
	first := composeOrFail(t, message)
	second := composeOrFail(t, message)
	if !bytes.Equal(first, second) {
		t.Fatal("same input and boundary source gave different output")
	}
	if !strings.Contains(string(first), "fixed-boundary-1") || !strings.Contains(string(first), "fixed-boundary-2") {
		t.Fatalf("boundaries not from the source:\n%s", first)
	}
}

func TestDefaultBoundariesAreRandom(t *testing.T) {
	message := baseMessage()
	message.HtmlBody = "<p>Hi</p>"
	first, err := compose.Composer{}.Compose(message)
	if err != nil {
		t.Fatal(err)
	}
	second, err := compose.Composer{}.Compose(message)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("two default compositions used the same boundary")
	}
	parsed := parse(t, first)
	reader := multipartReader(t, parsed.Header, parsed.Body, "multipart/alternative")
	nextPart(t, reader)
	nextPart(t, reader)
	assertNoMorePart(t, reader)
}

func TestRandomBoundariesDiffer(t *testing.T) {
	source := compose.RandomBoundaries{}
	if source.Next() == source.Next() {
		t.Fatal("random boundaries repeated")
	}
}

func TestCcOnlyIsEnoughRecipients(t *testing.T) {
	message := baseMessage()
	message.To = nil
	message.Cc = []compose.Address{{Email: "carol@example.org"}}
	names := strings.Join(headerNames(composeOrFail(t, message)), ",")
	if strings.Contains(names, "To,") || !strings.Contains(names, "Cc") {
		t.Fatalf("headers = %s", names)
	}
}

func TestMissingFromIsRefused(t *testing.T) {
	message := baseMessage()
	message.From = compose.Address{Name: "Nobody"}
	_, err := fixedComposer().Compose(message)
	assertMissing(t, err, "From")
}

func TestMissingRecipientsAreRefused(t *testing.T) {
	message := baseMessage()
	message.To = nil
	_, err := fixedComposer().Compose(message)
	assertMissing(t, err, "To")
}

func TestLineBreakInSubjectIsRefused(t *testing.T) {
	message := baseMessage()
	message.Subject = "Hi\r\nBcc: spy@example.net"
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "Subject")
}

func TestLineBreakInFromNameIsRefused(t *testing.T) {
	message := baseMessage()
	message.From.Name = "Ada\nBcc: spy@example.net"
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "From")
}

func TestLineBreakInRecipientEmailIsRefused(t *testing.T) {
	message := baseMessage()
	message.To = []compose.Address{{Email: "bob@example.org\r\nBcc: spy@example.net"}}
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "To")
}

func TestLineBreakInCcIsRefused(t *testing.T) {
	message := baseMessage()
	message.Cc = []compose.Address{{Name: "Carol\r", Email: "carol@example.org"}}
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "Cc")
}

func TestLineBreakInReplyToIsRefused(t *testing.T) {
	message := baseMessage()
	message.ReplyTo = []compose.Address{{Email: "reply@example.com\n"}}
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "Reply-To")
}

func TestLineBreakInMessageIdIsRefused(t *testing.T) {
	message := baseMessage()
	message.MessageId = "id@example.com\r\nX: y"
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "Message-ID")
}

func TestLineBreakInInReplyToIsRefused(t *testing.T) {
	message := baseMessage()
	message.InReplyTo = "id@example.com\n"
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "In-Reply-To")
}

func TestLineBreakInReferenceIsRefused(t *testing.T) {
	message := baseMessage()
	message.References = []string{"ok@example.com", "bad@example.com\r\n"}
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "References")
}

func TestLineBreakInAttachmentFileNameIsRefused(t *testing.T) {
	message := baseMessage()
	message.Attachments = []compose.Attachment{{FileName: "a\r\n.txt", ContentType: "text/plain"}}
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "Attachment")
}

func TestLineBreakInAttachmentContentTypeIsRefused(t *testing.T) {
	message := baseMessage()
	message.Attachments = []compose.Attachment{{FileName: "a.txt", ContentType: "text/plain\nX: y"}}
	_, err := fixedComposer().Compose(message)
	assertField(t, err, "Attachment")
}
