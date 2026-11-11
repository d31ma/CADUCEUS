package mailchecks

import (
	"errors"
	"testing"

	"mesail.dev/packages/mime"
)

type failingParser struct{}

func (failingParser) Headers(_ []byte) (mime.Headers, error) {
	return mime.Headers{}, errors.New("malformed")
}

func TestHeadersAreConvertedToTheWireModel(t *testing.T) {
	raw := []byte("From: Dan <dan@Outside.test>\r\nTo: Ana <ana@example.com>, bob@example.com\r\nCc: cy@example.org\r\nSubject: Hello\r\nMessage-ID: <m-1@outside.test>\r\nIn-Reply-To: <m-0@example.com>\r\nReferences: <m-0@example.com>\r\n\r\nHi\r\n")
	headers, err := NewMimeHeaderReader(mime.Parser{}).Headers(raw)
	if err != nil {
		t.Fatal(err)
	}
	if headers.MessageId != "m-1@outside.test" || headers.InReplyTo != "m-0@example.com" || headers.Subject != "Hello" {
		t.Fatalf("unexpected headers %+v", headers)
	}
	if headers.From.Name != "Dan" || len(headers.To) != 2 || headers.To[1].Address != "bob@example.com" || len(headers.Cc) != 1 || len(headers.References) != 1 {
		t.Fatalf("unexpected addresses %+v", headers)
	}
}

func TestMissingListsBecomeEmptyLists(t *testing.T) {
	headers, err := NewMimeHeaderReader(mime.Parser{}).Headers([]byte("Subject: Hello\r\n\r\nHi\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if headers.References == nil || headers.To == nil || headers.Cc == nil {
		t.Fatalf("lists must not be null: %+v", headers)
	}
}

func TestParserErrorsArePassedOn(t *testing.T) {
	if _, err := NewMimeHeaderReader(failingParser{}).Headers([]byte("x")); err == nil {
		t.Fatal("want an error")
	}
}
