//go:build blackbox

package blackbox

import (
	"net/smtp"
	"os"
	"strings"
	"testing"
)

func smtpAddress(t *testing.T) string {
	t.Helper()
	address := os.Getenv("MESAIL_SMTP_ADDR")
	if address == "" {
		t.Skip("MESAIL_SMTP_ADDR is not set")
	}
	return address
}

func TestAcIn1UnknownHostedRecipientIsRefusedAtRcpt(t *testing.T) {
	client, err := smtp.Dial(smtpAddress(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("dan@outside.test"); err != nil {
		t.Fatal(err)
	}
	err = client.Rcpt("nobody@example.com")
	if err == nil || !strings.HasPrefix(err.Error(), "550") {
		t.Fatalf("want 550 at RCPT TO, got %v", err)
	}
}

func TestAcIn1MailForAnUnhostedDomainIsNotRelayed(t *testing.T) {
	client, err := smtp.Dial(smtpAddress(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("dan@outside.test"); err != nil {
		t.Fatal(err)
	}
	if err := client.Rcpt("someone@not-hosted.test"); err == nil {
		t.Fatal("open relay: an unhosted recipient was accepted")
	}
}

func TestAcIn2HostedRecipientIsAccepted(t *testing.T) {
	client, err := smtp.Dial(smtpAddress(t))
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
	message := "From: dan@outside.test\r\nTo: ana@example.com\r\nSubject: Hello\r\nMessage-ID: <bb-1@outside.test>\r\n\r\nHi Ana\r\n"
	if _, err := writer.Write([]byte(message)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("DATA was not accepted: %v", err)
	}
}
