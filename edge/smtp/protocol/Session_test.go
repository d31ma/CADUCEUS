package protocol

import (
	"errors"
	"net/smtp"
	"strings"
	"testing"

	"mesail.dev/edge/smtp/reception"
)

func hostedMessage() string {
	return "From: dan@outside.test\r\nTo: ana@example.com\r\nSubject: Hello\r\nMessage-ID: <bb-1@outside.test>\r\n\r\nHi Ana\r\n"
}

func TestGreetingNamesTheHostAndMesail(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	if client.greeting != "mx.mesail.test ESMTP Mesail" {
		t.Fatalf("unexpected greeting %q", client.greeting)
	}
}

func TestUnknownHostedRecipientIsRefusedWith511(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<nobody@example.com>", 550, "5.1.1")
}

func TestUnhostedDomainIsNotRelayed(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<someone@not-hosted.test>", 550, "5.7.1 Relay not permitted")
}

func TestDirectoryFailureGivesTemporaryRefusal(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@failing.test>", 451, "4.4.3")
}

func TestHostedRecipientIsAcceptedAfterDelivery(t *testing.T) {
	server := newHarness(t, noChange)
	client, err := smtp.Dial(server.address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("dan@outside.test"); err != nil {
		t.Fatal(err)
	}
	if err := client.Rcpt("ana@example.com"); err != nil {
		t.Fatal(err)
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	writer.Write([]byte(hostedMessage()))
	if err := writer.Close(); err != nil {
		t.Fatalf("DATA was not accepted: %v", err)
	}
	delivered := server.delivery.delivered()
	if len(delivered) != 1 || string(delivered[0].Raw) != hostedMessage() || delivered[0].Recipients[0].DomainId != "example.com" {
		t.Fatalf("unexpected delivery %+v", delivered)
	}
}

func TestTransactionLogHasCodesButNoContent(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	client.sendData(hostedMessage())
	logs := server.logs.String()
	if !strings.Contains(logs, "reply=250") || !strings.Contains(logs, "inbound=in_1") || strings.Contains(logs, "Hi Ana") || strings.Contains(logs, "dan@outside.test") {
		t.Fatalf("unexpected log %q", logs)
	}
}

func TestDotStuffedLinesAndLineEndingsArriveUnchanged(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	content := "Subject: dots\r\n\r\n.leading dot\r\n..two dots\r\nend\r\n"
	code, _ := client.sendData(content)
	if code != 250 || string(server.delivery.delivered()[0].Raw) != content {
		t.Fatalf("got %d and %q", code, server.delivery.delivered()[0].Raw)
	}
}

func TestDeliveryFailureGivesTemporaryFailureAndNo250(t *testing.T) {
	server := newHarness(t, noChange)
	server.delivery.err = errors.New("mail-in answered HTTP 500")
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	code, message := client.sendData(hostedMessage())
	if code != 451 || !strings.HasPrefix(message, "4.3.0") {
		t.Fatalf("want 451 4.3.0, got %d %q", code, message)
	}
}

func TestPolicyRefusalGives550(t *testing.T) {
	server := newHarness(t, noChange)
	server.delivery.err = reception.Refused{}
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	code, message := client.sendData(hostedMessage())
	if code != 550 || !strings.HasPrefix(message, "5.7.1") {
		t.Fatalf("want 550 5.7.1, got %d %q", code, message)
	}
}

func TestDeclaredSizeOverTheLimitIsRefusedAtMail(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "SIZE 1024")
	client.expect("MAIL FROM:<dan@outside.test> SIZE=2048 BODY=8BITMIME", 552, "5.3.4")
	client.expect("MAIL FROM:<dan@outside.test> SIZE=512 BODY=8BITMIME", 250, "2.1.0")
}

func TestBadSizeParameterIsASyntaxError(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test> SIZE=many", 501, "5.5.4")
}

func TestMessageOverTheLimitIsRefusedDuringData(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	code, message := client.sendData("Subject: big\r\n\r\n" + strings.Repeat("0123456789abcdef\r\n", 100))
	if code != 552 || !strings.HasPrefix(message, "5.3.4") || len(server.delivery.delivered()) != 0 {
		t.Fatalf("want 552 5.3.4 and no delivery, got %d %q", code, message)
	}
	client.expect("NOOP", 250, "2.0.0")
	client.expect("RCPT TO:<ana@example.com>", 503, "Send MAIL first")
}

func TestTwoRecipientDomainsArriveInOneTransaction(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("RCPT TO:<cy@example.org>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	client.sendData(hostedMessage())
	recipients := server.delivery.delivered()[0].Recipients
	if len(recipients) != 2 || recipients[1].DomainId != "example.org" {
		t.Fatalf("unexpected recipients %+v", recipients)
	}
}

func TestDuplicateRecipientIsIgnored(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("RCPT TO:<ANA@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	client.sendData(hostedMessage())
	if len(server.delivery.delivered()[0].Recipients) != 1 {
		t.Fatalf("duplicate was kept: %+v", server.delivery.delivered()[0].Recipients)
	}
}

func TestRecipientLimitIsOneHundred(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.pipelineRecipients(100)
	client.expect("RCPT TO:<ana@example.com>", 452, "4.5.3")
}

func TestNullSenderIsAllowed(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("HELO outside.test", 250, "mx.mesail.test")
	client.expect("MAIL FROM:<>", 250, "2.1.0")
	client.expect("RCPT TO:<ana@example.com>", 250, "2.1.5")
	client.expect("DATA", 354, "")
	client.sendData(hostedMessage())
	if server.delivery.delivered()[0].EnvelopeFrom != "" {
		t.Fatal("null sender was changed")
	}
}

func TestCommandsInTheWrongOrderGive503(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("MAIL FROM:<dan@outside.test>", 503, "5.5.1")
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("RCPT TO:<ana@example.com>", 503, "5.5.1")
	client.expect("DATA", 503, "5.5.1")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("MAIL FROM:<dan@outside.test>", 503, "5.5.1")
	client.expect("DATA", 503, "5.5.1")
}

func TestSyntaxErrorsGive501(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("EHLO", 501, "5.5.4")
	client.expect("EHLO outside.test", 250, "Hello")
	client.expect("MAIL dan@outside.test", 501, "5.5.4")
	client.expect("MAIL FROM:<dan@outside.test", 501, "5.5.4")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RCPT TO:<nobody>", 501, "5.5.4")
	client.expect("RCPT TO:<>", 501, "5.5.4")
}

func TestUnknownCommandGives500(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("BDAT 10", 500, "5.5.2")
}

func TestLongLineGives500AndTheSessionContinues(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("NOOP "+strings.Repeat("a", 5000), 500, "5.5.2 Line too long")
	client.expect("NOOP "+strings.Repeat("a", 1000), 500, "5.5.2 Line too long")
	client.expect("NOOP", 250, "2.0.0")
}

func TestAuthIsNotOffered(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	message := client.expect("EHLO outside.test", 250, "PIPELINING")
	if strings.Contains(message, "AUTH") || strings.Contains(message, "STARTTLS") {
		t.Fatalf("unexpected extensions %q", message)
	}
	client.expect("AUTH PLAIN AGFuYQBzZWNyZXQ=", 502, "5.5.1")
	client.expect("STARTTLS", 502, "5.5.1")
}

func TestSmallCommandsAnswerAsExpected(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.expect("VRFY ana", 252, "2.5.0")
	client.expect("HELP", 214, "2.0.0")
	client.expect("NOOP", 250, "2.0.0")
	client.expect("EHLO outside.test", 250, "ENHANCEDSTATUSCODES")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("RSET", 250, "2.0.0")
	client.expect("MAIL FROM:<dan@outside.test>", 250, "2.1.0")
	client.expect("QUIT", 221, "2.0.0")
}

func TestTooManyErrorsCloseTheConnection(t *testing.T) {
	server := newHarness(t, noChange)
	client := dialRaw(t, server.address)
	client.repeat("WHAT", 9)
	client.expect("WHAT", 500, "5.5.2")
	code, message := client.response()
	if code != 421 || !strings.HasPrefix(message, "4.7.0") {
		t.Fatalf("want 421 4.7.0, got %d %q", code, message)
	}
}
