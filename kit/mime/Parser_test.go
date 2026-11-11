package mime

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestHeadersReadTheAddressesWithALowercaseDomain(t *testing.T) {
	headers, err := Parser{}.Headers(testMessages{}.plain())
	from := Mailbox{Name: "Ana Lima", Address: "Ana@example.com"}
	to := []Mailbox{{Name: "Bo", Address: "bo@mesail.dev"}, {Address: "carl@example.org"}}
	cc := []Mailbox{{Name: "Dee, D.", Address: "dee@example.net"}}
	if err != nil || headers.From != from || !reflect.DeepEqual(headers.To, to) || !reflect.DeepEqual(headers.Cc, cc) {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersReadTheIdsWithoutAngleBrackets(t *testing.T) {
	headers, err := Parser{}.Headers(testMessages{}.plain())
	references := []string{"r1@example.com", "r2@example.com", "r3@example.com", "r4@example.com"}
	if err != nil || headers.MessageId != "m1.abc@example.com" || headers.InReplyTo != "m0.xyz@example.com" || !reflect.DeepEqual(headers.References, references) {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersKeepTheSubjectAndDate(t *testing.T) {
	headers, err := Parser{}.Headers(testMessages{}.plain())
	if err != nil || headers.Subject != "Lunch on Friday" || headers.Date != "Tue, 10 Nov 2026 12:00:00 +0000" {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersDecodeAUtf8BaseSubject(t *testing.T) {
	headers, err := Parser{}.Headers([]byte(testMessages{}.encodedSubjects()[0]))
	if err != nil || headers.Subject != "Café menu" {
		t.Fatalf("got %q, %v", headers.Subject, err)
	}
}

func TestHeadersDecodeAUtf8QuotedSubject(t *testing.T) {
	headers, err := Parser{}.Headers([]byte(testMessages{}.encodedSubjects()[1]))
	if err != nil || headers.Subject != "Café menu" {
		t.Fatalf("got %q, %v", headers.Subject, err)
	}
}

func TestHeadersDecodeALatin1Subject(t *testing.T) {
	headers, err := Parser{}.Headers([]byte(testMessages{}.encodedSubjects()[2]))
	if err != nil || headers.Subject != "Café menu" {
		t.Fatalf("got %q, %v", headers.Subject, err)
	}
}

func TestHeadersDecodeAWindows1252Subject(t *testing.T) {
	headers, err := Parser{}.Headers([]byte(testMessages{}.encodedSubjects()[3]))
	if err != nil || headers.Subject != "Café menu" {
		t.Fatalf("got %q, %v", headers.Subject, err)
	}
}

func TestHeadersKeepASubjectWithAnUnknownEncoding(t *testing.T) {
	headers, err := Parser{}.Headers([]byte("Subject: =?utf-8?X?abc?=\r\n\r\n"))
	if err != nil || headers.Subject != "=?utf-8?X?abc?=" {
		t.Fatalf("got %q, %v", headers.Subject, err)
	}
}

func TestHeadersDecodeAnEncodedDisplayName(t *testing.T) {
	headers, err := Parser{}.Headers([]byte("From: =?UTF-8?Q?Jos=C3=A9?= <jose@EXAMPLE.com>\r\n\r\n"))
	if err != nil || headers.From != (Mailbox{Name: "José", Address: "jose@example.com"}) {
		t.Fatalf("got %+v, %v", headers.From, err)
	}
}

func TestHeadersTolerateLineFeedOnlyLineEnds(t *testing.T) {
	lineFeeds := []byte(strings.ReplaceAll(string(testMessages{}.plain()), "\r\n", "\n"))
	headers, err := Parser{}.Headers(lineFeeds)
	want, _ := Parser{}.Headers(testMessages{}.plain())
	if err != nil || !reflect.DeepEqual(headers, want) {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersTolerateAMissingBody(t *testing.T) {
	headers, err := Parser{}.Headers([]byte("Subject: only\r\nFrom: ana@example.com"))
	if err != nil || headers.Subject != "only" || headers.From.Address != "ana@example.com" {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersGiveEmptyValuesForMissingHeaders(t *testing.T) {
	headers, err := Parser{}.Headers([]byte("X-Other: 1\r\n\r\nbody"))
	if err != nil || !reflect.DeepEqual(headers, Headers{}) {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersGiveAnEmptyListForABadAddressList(t *testing.T) {
	headers, err := Parser{}.Headers([]byte("From: not an address\r\nTo: <<<\r\n\r\n"))
	if err != nil || headers.From != (Mailbox{}) || headers.To != nil {
		t.Fatalf("got %+v, %v", headers, err)
	}
}

func TestHeadersRefuseAnUnreadableHeaderBlock(t *testing.T) {
	_, err := Parser{}.Headers([]byte(" starts with a space\r\n\r\n"))
	if !errors.Is(err, Malformed{}) || err.Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestHeadersRefuseAnEmptyMessage(t *testing.T) {
	_, err := Parser{}.Headers(nil)
	if !errors.Is(err, Malformed{}) {
		t.Fatalf("got %v", err)
	}
}

func TestParseReadsAPlainTextMessage(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.plain())
	if err != nil || message.Text != "Hello Bo,\r\nsee you at noon.\r\n" || message.Html != "" || len(message.Attachments) != 0 {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseKeepsTheSameHeadersAsHeaders(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.plain())
	headers, _ := Parser{}.Headers(testMessages{}.plain())
	if err != nil || !reflect.DeepEqual(message.Headers, headers) {
		t.Fatalf("got %+v, %v", message.Headers, err)
	}
}

func TestParsePicksTextAndHtmlFromAnAlternative(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.alternative())
	if err != nil || message.Text != "Plain report." || message.Html != "<p>Html report.</p>" || len(message.Attachments) != 0 {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseDecodesABase64Attachment(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.mixed())
	want := Attachment{FileName: "invoice.pdf", MediaType: "application/pdf", Size: 18, Content: []byte("%PDF-1.4\n% invoice")}
	if err != nil || message.Text != "See the invoice." || len(message.Attachments) != 1 || !reflect.DeepEqual(message.Attachments[0], want) {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseWalksNestedMultiparts(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.nested())
	if err != nil || message.Text != "Café menu" || message.Html != "<p>“Café” <img src=\"cid:logo\"></p>" {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseMarksARelatedImageInline(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.nested())
	image := message.Attachments[0]
	if err != nil || image.MediaType != "image/png" || image.ContentId != "logo@example.com" || !image.Inline || !bytes.HasPrefix(image.Content, []byte("\x89PNG")) {
		t.Fatalf("got %+v, %v", image, err)
	}
}

func TestParseKeepsAnAttachedMessageUnparsed(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.nested())
	attached := message.Attachments[1]
	if err != nil || len(message.Attachments) != 2 || attached.MediaType != "message/rfc822" || attached.Inline || !bytes.Contains(attached.Content, []byte("Subject: Old")) {
		t.Fatalf("got %+v, %v", message.Attachments, err)
	}
}

func TestParseDecodesAnRfc2231FileName(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.fileNames())
	if err != nil || message.Attachments[0].FileName != "naïve plan.txt" || message.Attachments[0].MediaType != "text/plain" {
		t.Fatalf("got %+v, %v", message.Attachments, err)
	}
}

func TestParseDecodesAnEncodedWordFileName(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.fileNames())
	if err != nil || message.Attachments[1].FileName != "été.bin" || string(message.Attachments[1].Content) != "data" {
		t.Fatalf("got %+v, %v", message.Attachments, err)
	}
}

func TestParseUsesTheFirstTextPartAsText(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.fileNames())
	if err != nil || message.Text != "Second text part." || len(message.Attachments) != 2 {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseKeepsALaterTextPartAsAnAttachment(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.alternative())
	extra := append(testMessages{}.alternative()[:0:0], testMessages{}.alternative()...)
	extra = bytes.Replace(extra, []byte("--alt-1--"), []byte("--alt-1\r\nContent-Type: text/html\r\n\r\n<p>again</p>\r\n--alt-1--"), 1)
	again, againErr := Parser{}.Parse(extra)
	if err != nil || againErr != nil || len(again.Attachments) != 1 || string(again.Attachments[0].Content) != "<p>again</p>" || again.Html != message.Html {
		t.Fatalf("got %+v, %v", again, againErr)
	}
}

func TestParseDecodesAQuotedPrintableBody(t *testing.T) {
	raw := testMessages{}.crlf("Content-Type: text/plain; charset=utf-8\nContent-Transfer-Encoding: Quoted-Printable\n\nA long line that is =\nsoft broken and has caf=C3=A9.\n")
	message, err := Parser{}.Parse(raw)
	if err != nil || message.Text != "A long line that is soft broken and has café.\r\n" {
		t.Fatalf("got %q, %v", message.Text, err)
	}
}

func TestParseDecodesBase64WithoutPaddingAndWithNoise(t *testing.T) {
	raw := []byte("Content-Type: text/plain\r\nContent-Transfer-Encoding: base64\r\n\r\nSGVs bG8*\r\n")
	message, err := Parser{}.Parse(raw)
	if err != nil || message.Text != "Hello" {
		t.Fatalf("got %q, %v", message.Text, err)
	}
}

func TestParseDecodesALatin1Body(t *testing.T) {
	raw := []byte("Content-Type: text/plain; charset=latin1\r\n\r\nna\xefve")
	message, err := Parser{}.Parse(raw)
	if err != nil || message.Text != "naïve" {
		t.Fatalf("got %q, %v", message.Text, err)
	}
}

func TestParseReplacesInvalidBytesOfAnUnknownCharset(t *testing.T) {
	raw := []byte("Content-Type: text/plain; charset=x-unknown\r\n\r\nok \xff end")
	message, err := Parser{}.Parse(raw)
	if err != nil || message.Text != "ok � end" {
		t.Fatalf("got %q, %v", message.Text, err)
	}
}

func TestParseTreatsAMissingContentTypeAsPlainText(t *testing.T) {
	message, err := Parser{}.Parse([]byte("Subject: hi\n\nplain body\n"))
	if err != nil || message.Text != "plain body\n" {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseReadsLineFeedOnlyMultiparts(t *testing.T) {
	lineFeeds := []byte(strings.ReplaceAll(string(testMessages{}.alternative()), "\r\n", "\n"))
	message, err := Parser{}.Parse(lineFeeds)
	if err != nil || message.Text != "Plain report." || message.Html != "<p>Html report.</p>" {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseKeepsAMultipartWithoutABoundaryAsAnAttachment(t *testing.T) {
	message, err := Parser{}.Parse([]byte("Content-Type: multipart/mixed\r\n\r\nbody"))
	if err != nil || len(message.Attachments) != 1 || message.Attachments[0].MediaType != "multipart/mixed" {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseRefusesAMessageOverTheSizeLimit(t *testing.T) {
	raw := append([]byte("Subject: big\r\n\r\n"), make([]byte, Parser{}.MaxSize())...)
	_, err := Parser{}.Parse(raw)
	if !errors.Is(err, TooLarge{}) || err.Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestParseRefusesAHeaderBlockOverTheLimit(t *testing.T) {
	raw := []byte("X-Long: " + strings.Repeat("a", Parser{}.MaxHeaderSize()) + "\r\n\r\nbody")
	_, err := Parser{}.Parse(raw)
	if !errors.Is(err, TooLarge{}) {
		t.Fatalf("got %v", err)
	}
}

func TestParseAcceptsNestingAtTheDepthLimit(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.deep(Parser{}.MaxDepth()))
	if err != nil || message.Text != "bottom" {
		t.Fatalf("got %+v, %v", message, err)
	}
}

func TestParseRefusesNestingOverTheDepthLimit(t *testing.T) {
	_, err := Parser{}.Parse(testMessages{}.deep(Parser{}.MaxDepth() + 1))
	if !errors.Is(err, TooDeep{}) || err.Error() == "" {
		t.Fatalf("got %v", err)
	}
}

func TestParseAcceptsPartsAtThePartLimit(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.wide(Parser{}.MaxParts() - 1))
	if err != nil || len(message.Attachments) != (Parser{}).MaxParts()-1 {
		t.Fatalf("got %d attachments, %v", len(message.Attachments), err)
	}
}

func TestParseRefusesPartsOverThePartLimit(t *testing.T) {
	message, err := Parser{}.Parse(testMessages{}.wide(Parser{}.MaxParts()))
	if !errors.Is(err, TooManyParts{}) || err.Error() == "" || len(message.Attachments) != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestParseRefusesAnUnreadableHeaderBlock(t *testing.T) {
	_, err := Parser{}.Parse([]byte("no colon here\r\n\r\nbody"))
	if !errors.Is(err, Malformed{}) {
		t.Fatalf("got %v", err)
	}
}
