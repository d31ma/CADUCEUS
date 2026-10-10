package mailprovider

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestPasswordValidatorAuthenticatesIMAPInboxAndSMTPWithoutSendingMail(t *testing.T) {
	imapAddress, stopIMAP := serveProtocol(t, imapSuccess)
	defer stopIMAP()
	smtpAddress, stopSMTP := serveProtocol(t, smtpSuccess)
	defer stopSMTP()
	validator := localPasswordValidator(t, 2*time.Second)

	capabilities, err := validator.Validate(context.Background(), validationRequest(imapAddress, smtpAddress, "top-secret"))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !capabilities.MailRead || !capabilities.MailWrite || !capabilities.Send || capabilities.Labels {
		t.Fatalf("Validate() capabilities = %#v", capabilities)
	}
}

func TestPasswordValidatorUsesDelegatedXOAUTH2ForMicrosoftSharedMailbox(t *testing.T) {
	const shared = "support@example.com"
	const token = "microsoft-access-token"
	assertXOAUTH2 := func(line string) {
		fields := strings.Fields(strings.TrimSpace(line))
		encoded := fields[len(fields)-1]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil || string(decoded) != "user="+shared+"\x01auth=Bearer "+token+"\x01\x01" {
			t.Errorf("XOAUTH2 payload = %q, %v", decoded, err)
		}
	}
	IMAPAddress, stopIMAP := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK ready\r\n")
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "a001 AUTHENTICATE XOAUTH2 ") {
			t.Errorf("IMAP auth = %q", line)
		} else {
			assertXOAUTH2(line)
		}
		_, _ = fmt.Fprint(connection, "a001 OK authenticated\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "* 0 EXISTS\r\na002 OK [READ-WRITE] selected\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "* BYE\r\na003 OK logout\r\n")
	})
	defer stopIMAP()
	SMTPAddress, stopSMTP := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 ready\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250-fixture\r\n250 AUTH XOAUTH2\r\n")
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "AUTH XOAUTH2 ") {
			t.Errorf("SMTP auth = %q", line)
		} else {
			assertXOAUTH2(line)
		}
		_, _ = fmt.Fprint(connection, "235 authenticated\r\n")
		line, _ = reader.ReadString('\n')
		if line != "MAIL FROM:<support@example.com>\r\n" {
			t.Errorf("SMTP sender validation = %q", line)
		}
		_, _ = fmt.Fprint(connection, "250 sender permitted\r\n")
		line, _ = reader.ReadString('\n')
		if line != "RSET\r\n" {
			t.Errorf("SMTP reset = %q", line)
		}
		_, _ = fmt.Fprint(connection, "250 reset\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "221 bye\r\n")
	})
	defer stopSMTP()
	request := validationRequest(IMAPAddress, SMTPAddress, "")
	request.Provider = accounts.ProviderMicrosoft
	request.MailboxAddress = shared
	request.AuthIdentity = "delegate@example.com"
	request.SharedMailboxAddress = shared
	request.Credentials = accounts.Credentials{AccessToken: token, RefreshToken: "refresh", ExpiresAt: time.Now().Add(time.Hour)}

	capabilities, err := localPasswordValidator(t, time.Second).Validate(context.Background(), request)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !capabilities.MailRead || !capabilities.MailWrite || !capabilities.Send || !capabilities.SharedMailbox {
		t.Fatalf("capabilities = %#v", capabilities)
	}
}

func TestPasswordValidatorRejectsMicrosoftSharedMailboxWithoutSendAs(t *testing.T) {
	IMAPAddress, stopIMAP := serveProtocol(t, imapSuccess)
	defer stopIMAP()
	SMTPAddress, stopSMTP := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 ready\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250-fixture\r\n250 AUTH XOAUTH2\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "235 authenticated\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "550 5.7.60 Client does not have permissions to send as this sender\r\n")
	})
	defer stopSMTP()
	request := validationRequest(IMAPAddress, SMTPAddress, "")
	request.Provider = accounts.ProviderMicrosoft
	request.MailboxAddress = "support@example.com"
	request.AuthIdentity = "delegate@example.com"
	request.SharedMailboxAddress = request.MailboxAddress
	request.Credentials = accounts.Credentials{AccessToken: "access-token"}

	_, err := localPasswordValidator(t, time.Second).Validate(context.Background(), request)
	if !accounts.IsValidationFailure(err, accounts.ValidationSMTP, accounts.ValidationAuthentication) {
		t.Fatalf("Validate() error = %v, want send-as rejection", err)
	}
}

func TestPasswordValidatorUsesXOAUTH2AndLabelCapabilityForGmail(t *testing.T) {
	IMAPAddress, stopIMAP := serveProtocol(t, imapSuccess)
	defer stopIMAP()
	SMTPAddress, stopSMTP := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "220 ready\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250-fixture\r\n250 AUTH XOAUTH2\r\n")
		line, _ := reader.ReadString('\n')
		if !strings.HasPrefix(line, "AUTH XOAUTH2 ") {
			t.Errorf("Gmail auth = %q", line)
		}
		_, _ = fmt.Fprint(connection, "235 authenticated\r\n")
		line, _ = reader.ReadString('\n')
		if line != "MAIL FROM:<owner@gmail.com>\r\n" {
			t.Errorf("Gmail sender = %q", line)
		}
		_, _ = fmt.Fprint(connection, "250 sender permitted\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "250 reset\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "221 bye\r\n")
	})
	defer stopSMTP()
	request := validationRequest(IMAPAddress, SMTPAddress, "")
	request.Provider = accounts.ProviderGmail
	request.MailboxAddress = "owner@gmail.com"
	request.AuthIdentity = request.MailboxAddress
	request.Credentials = accounts.Credentials{AccessToken: "google-access-token"}
	capabilities, err := localPasswordValidator(t, time.Second).Validate(context.Background(), request)
	if err != nil || !capabilities.MailRead || !capabilities.Send || !capabilities.Labels || capabilities.SharedMailbox {
		t.Fatalf("Validate() = %#v, %v", capabilities, err)
	}
}

func TestPasswordValidatorClassifiesAuthenticationFailureWithoutLeakingSecrets(t *testing.T) {
	imapAddress, stopIMAP := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK ready\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "a001 NO password top-secret rejected by upstream\r\n")
	})
	defer stopIMAP()
	smtpAddress, stopSMTP := serveProtocol(t, smtpSuccess)
	defer stopSMTP()

	_, err := localPasswordValidator(t, time.Second).Validate(context.Background(), validationRequest(imapAddress, smtpAddress, "top-secret"))
	if !accounts.IsValidationFailure(err, accounts.ValidationIMAP, accounts.ValidationAuthentication) {
		t.Fatalf("Validate() error = %v, want IMAP authentication failure", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "top-secret") || strings.Contains(strings.ToLower(err.Error()), "upstream") {
		t.Fatalf("Validate() leaked server or credential detail: %v", err)
	}
}

func TestPasswordValidatorRequiresInboxAndNeverFallsThroughToSMTP(t *testing.T) {
	imapAddress, stopIMAP := serveProtocol(t, func(connection net.Conn) {
		reader := bufio.NewReader(connection)
		_, _ = fmt.Fprint(connection, "* OK ready\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "a001 OK authenticated\r\n")
		_, _ = reader.ReadString('\n')
		_, _ = fmt.Fprint(connection, "a002 NO mailbox does not exist\r\n")
	})
	defer stopIMAP()
	smtpAddress, stopSMTP := serveProtocol(t, func(net.Conn) {
		t.Error("SMTP was contacted after IMAP validation failed")
	})
	defer stopSMTP()

	_, err := localPasswordValidator(t, time.Second).Validate(context.Background(), validationRequest(imapAddress, smtpAddress, "secret"))
	if !accounts.IsValidationFailure(err, accounts.ValidationIMAP, accounts.ValidationMailbox) {
		t.Fatalf("Validate() error = %v, want missing mailbox", err)
	}
}

func TestPasswordValidatorRejectsUnsafeResolutionAndPinsApprovedAddresses(t *testing.T) {
	validator, err := NewPasswordValidator(time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	validator.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}}}
	if _, err := validator.resolveEndpoint(context.Background(), &accounts.Endpoint{Host: "metadata.example", Port: 993}); !errors.Is(err, ErrUnsafeEndpoint) {
		t.Fatalf("resolveEndpoint(link-local) error = %v", err)
	}

	validator, err = NewPasswordValidator(time.Second, []string{"10.20.0.0/16"})
	if err != nil {
		t.Fatal(err)
	}
	validator.resolver = staticResolver{addresses: []net.IPAddr{{IP: net.ParseIP("10.20.30.40")}}}
	addresses, err := validator.resolveEndpoint(context.Background(), &accounts.Endpoint{Host: "mail.internal.example", Port: 993})
	if err != nil || len(addresses) != 1 || addresses[0].String() != "10.20.30.40" {
		t.Fatalf("resolveEndpoint(approved) = %#v, %v", addresses, err)
	}
}

func localPasswordValidator(t *testing.T, timeout time.Duration) *PasswordValidator {
	t.Helper()
	validator, err := NewPasswordValidator(timeout, []string{"127.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	return validator
}

type staticResolver struct{ addresses []net.IPAddr }

func (resolver staticResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return resolver.addresses, nil
}

func validationRequest(imapAddress, smtpAddress, password string) accounts.ValidationRequest {
	imapHost, imapPort, _ := net.SplitHostPort(imapAddress)
	smtpHost, smtpPort, _ := net.SplitHostPort(smtpAddress)
	return accounts.ValidationRequest{
		Provider: accounts.ProviderIMAP, MailboxAddress: "alice@remote.example",
		Credentials: accounts.Credentials{Username: "alice@remote.example", Password: password},
		IMAP:        &accounts.Endpoint{Host: imapHost, Port: parsePort(imapPort), Security: accounts.SecurityNone},
		SMTP:        &accounts.Endpoint{Host: smtpHost, Port: parsePort(smtpPort), Security: accounts.SecurityNone},
	}
}

func parsePort(value string) uint16 {
	var port uint16
	_, _ = fmt.Sscan(value, &port)
	return port
}

func serveProtocol(t *testing.T, handler func(net.Conn)) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			handler(connection)
		}
	}()
	return listener.Addr().String(), func() {
		_ = listener.Close()
		<-done
	}
}

func imapSuccess(connection net.Conn) {
	reader := bufio.NewReader(connection)
	_, _ = fmt.Fprint(connection, "* OK ready\r\n")
	_, _ = reader.ReadString('\n')
	_, _ = fmt.Fprint(connection, "a001 OK authenticated\r\n")
	_, _ = reader.ReadString('\n')
	_, _ = fmt.Fprint(connection, "* 0 EXISTS\r\na002 OK [READ-WRITE] selected\r\n")
	_, _ = reader.ReadString('\n')
	_, _ = fmt.Fprint(connection, "* BYE\r\na003 OK logout\r\n")
}

func smtpSuccess(connection net.Conn) {
	reader := bufio.NewReader(connection)
	_, _ = fmt.Fprint(connection, "220 smtp.remote.example ESMTP\r\n")
	_, _ = reader.ReadString('\n')
	_, _ = fmt.Fprint(connection, "250-smtp.remote.example\r\n250 AUTH PLAIN\r\n")
	_, _ = reader.ReadString('\n')
	_, _ = fmt.Fprint(connection, "235 2.7.0 authenticated\r\n")
	line, _ := reader.ReadString('\n')
	if strings.HasPrefix(strings.ToUpper(line), "MAIL FROM") {
		_, _ = fmt.Fprint(connection, "550 validation must not send mail\r\n")
		return
	}
	_, _ = fmt.Fprint(connection, "221 bye\r\n")
}
