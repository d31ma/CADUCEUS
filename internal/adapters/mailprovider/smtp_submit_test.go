package mailprovider

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsend"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestSMTPConnectorAuthenticatesSubmitsAndDotStuffs(t *testing.T) {
	var data string
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 fixture ESMTP\r\n")
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "EHLO ") {
			t.Errorf("EHLO = %q", line)
		}
		_, _ = fmt.Fprint(connection, "250-fixture\r\n250 AUTH PLAIN\r\n")
		line, _ = reader.ReadString('\n')
		if !strings.HasPrefix(line, "AUTH PLAIN ") {
			t.Errorf("AUTH = %q", line)
		}
		_, _ = fmt.Fprint(connection, "235 authenticated\r\n")
		line, _ = reader.ReadString('\n')
		if line != "MAIL FROM:<remote@remote.example>\r\n" {
			t.Errorf("MAIL = %q", line)
		}
		_, _ = fmt.Fprint(connection, "250 sender ok\r\n")
		line, _ = reader.ReadString('\n')
		if line != "RCPT TO:<recipient@example.net>\r\n" {
			t.Errorf("RCPT = %q", line)
		}
		_, _ = fmt.Fprint(connection, "250 recipient ok\r\n")
		line, _ = reader.ReadString('\n')
		if line != "DATA\r\n" {
			t.Errorf("DATA = %q", line)
		}
		_, _ = fmt.Fprint(connection, "354 continue\r\n")
		var lines []string
		for {
			line, _ = reader.ReadString('\n')
			if line == ".\r\n" {
				break
			}
			lines = append(lines, line)
		}
		data = strings.Join(lines, "")
		_, _ = fmt.Fprint(connection, "250 accepted\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "221 bye\r\n")
	})
	defer stop()
	connector, _ := NewSMTPConnector(localPasswordValidator(t, 2*time.Second))
	result, err := connector.Submit(context.Background(), smtpContext(address, []byte("From: remote@remote.example\r\nTo: recipient@example.net\r\nMessage-ID: <one@remote.example>\r\n\r\n.leading dot\r\n")))
	if err != nil || result.Outcome != accountsend.OutcomeSent {
		t.Fatalf("Submit() = %#v, %v", result, err)
	}
	if !strings.Contains(data, "..leading dot\r\n") {
		t.Fatalf("SMTP data was not dot-stuffed: %q", data)
	}
}

func TestSMTPConnectorDoesNotRetryAmbiguousDataAcceptance(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 fixture ESMTP\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250-fixture\r\n250 AUTH PLAIN\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "235 ok\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250 ok\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250 ok\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "354 continue\r\n")
		for {
			line, _ := reader.ReadString('\n')
			if line == ".\r\n" {
				break
			}
		}
		// Close without the post-DATA reply: the provider may have accepted it.
	})
	defer stop()
	connector, _ := NewSMTPConnector(localPasswordValidator(t, 2*time.Second))
	result, err := connector.Submit(context.Background(), smtpContext(address, []byte("From: remote@remote.example\r\nMessage-ID: <one@remote.example>\r\n\r\nhello\r\n")))
	if err != nil || result.Outcome != accountsend.OutcomeAmbiguous {
		t.Fatalf("Submit() = %#v, %v", result, err)
	}
}

func TestSMTPConnectorSendsAsMicrosoftSharedMailboxWithXOAUTH2(t *testing.T) {
	address, stop := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 fixture ESMTP\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250-fixture\r\n250 AUTH XOAUTH2\r\n")
		line, _ := reader.ReadString('\n')
		fields := strings.Fields(line)
		decoded, err := base64.StdEncoding.DecodeString(fields[len(fields)-1])
		if err != nil || string(decoded) != "user=support@example.com\x01auth=Bearer access-token\x01\x01" {
			t.Errorf("AUTH = %q decoded=%q err=%v", line, decoded, err)
		}
		_, _ = fmt.Fprint(connection, "235 authenticated\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250 sender ok\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250 recipient ok\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "354 continue\r\n")
		for {
			line, _ = reader.ReadString('\n')
			if line == ".\r\n" {
				break
			}
		}
		_, _ = fmt.Fprint(connection, "250 accepted\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "221 bye\r\n")
	})
	defer stop()
	host, port, _ := net.SplitHostPort(address)
	request := smtpContext(address, []byte("From: support@example.com\r\nTo: recipient@example.net\r\nMessage-ID: <one@example.com>\r\n\r\nhello\r\n"))
	request.Account.Provider = accounts.ProviderMicrosoft
	request.Account.MailboxAddress = "support@example.com"
	request.Account.AuthIdentity = stringPointer("delegate@example.com")
	request.Account.SharedMailboxAddress = stringPointer("support@example.com")
	request.Account.Connection.SMTP = &accounts.Endpoint{Host: host, Port: parsePort(port), Security: accounts.SecurityNone}
	request.Credentials = accounts.Credentials{AccessToken: "access-token", RefreshToken: "refresh-token", ExpiresAt: time.Now().Add(time.Hour)}
	request.Submission.EnvelopeFrom = "support@example.com"

	connector, _ := NewSMTPConnector(localPasswordValidator(t, time.Second))
	result, err := connector.Submit(context.Background(), request)
	if err != nil || result.Outcome != accountsend.OutcomeSent {
		t.Fatalf("Submit() = %#v, %v", result, err)
	}
}

func TestSMTPConnectorRejectsHeaderSpoofBeforeConnecting(t *testing.T) {
	connector, _ := NewSMTPConnector(localPasswordValidator(t, time.Second))
	request := smtpContext("127.0.0.1:9", []byte("From: attacker@example.net\r\nMessage-ID: <one@example.net>\r\n\r\nhello\r\n"))
	result, err := connector.Submit(context.Background(), request)
	if err != nil || result.Outcome != accountsend.OutcomeRejected || result.Code != "invalid_message" {
		t.Fatalf("Submit() = %#v, %v", result, err)
	}
}

func smtpContext(address string, raw []byte) accountsend.Context {
	host, port, _ := net.SplitHostPort(address)
	var parsed uint16
	_, _ = fmt.Sscan(port, &parsed)
	owner := accounts.Owner{TenantID: "tenant", PrincipalID: "alice", Address: "alice@example.com"}
	return accountsend.Context{Account: accounts.ExternalAccount{ID: "ext_AAAAAAAAAAAAAAAAAAAA", Owner: owner, Provider: accounts.ProviderIMAP,
		MailboxAddress: "remote@remote.example", Connection: accounts.ConnectionSettings{SMTP: &accounts.Endpoint{Host: host, Port: parsed, Security: accounts.SecurityNone}}},
		Credentials: accounts.Credentials{Username: "remote@remote.example", Password: "secret"}, Submission: accountsend.Submission{
			ID: "sub_abcdefghijklmnopqrst", Scope: accountsync.Scope{Owner: owner, AccountID: "ext_AAAAAAAAAAAAAAAAAAAA"}, EnvelopeFrom: "remote@remote.example",
			Recipients: []string{"recipient@example.net"}, RawRFC822: raw}}
}

func stringPointer(value string) *string { return &value }
