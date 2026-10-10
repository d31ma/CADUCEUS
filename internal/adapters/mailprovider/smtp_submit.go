package mailprovider

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsend"
)

type SMTPConnector struct {
	transport *PasswordValidator
}

func NewSMTPConnector(transport *PasswordValidator) (*SMTPConnector, error) {
	if transport == nil {
		return nil, errors.New("SMTP submission transport is required")
	}
	return &SMTPConnector{transport: transport}, nil
}

func (connector *SMTPConnector) Submit(ctx context.Context, request accountsend.Context) (accountsend.Result, error) {
	if request.Account.Provider != accounts.ProviderIMAP && !oauthMailProvider(request.Account.Provider) || request.Account.Connection.SMTP == nil ||
		request.Submission.EnvelopeFrom != request.Account.MailboxAddress || len(request.Submission.Recipients) == 0 ||
		!validSubmissionSource(request.Submission.RawRFC822, request.Submission.EnvelopeFrom) {
		return accountsend.Result{Outcome: accountsend.OutcomeRejected, Code: "invalid_message"}, nil
	}
	endpoint := request.Account.Connection.SMTP
	connection, err := connector.transport.connect(ctx, endpoint)
	if err != nil {
		return accountsend.Result{}, err
	}
	defer connection.Close()
	reader := protocolReader(connection)
	if code, _ := readSMTPReply(reader); code != 220 {
		return smtpSubmissionReply(code, "greeting_rejected"), nil
	}
	capabilities, err := smtpHello(connection, reader)
	if err != nil {
		return accountsend.Result{}, err
	}
	if endpoint.Security == accounts.SecurityStartTLS {
		if !strings.Contains(strings.ToUpper(capabilities), "STARTTLS") {
			return accountsend.Result{Outcome: accountsend.OutcomeRejected, Code: "starttls_unavailable"}, nil
		}
		connection, reader, err = connector.transport.startSMTPTLS(ctx, connection, reader, endpoint)
		if err != nil {
			return accountsend.Result{}, err
		}
		capabilities, err = smtpHello(connection, reader)
		if err != nil {
			return accountsend.Result{}, err
		}
	}
	authCommand := ""
	if oauthMailProvider(request.Account.Provider) {
		if !strings.Contains(strings.ToUpper(capabilities), "XOAUTH2") || request.Credentials.AccessToken == "" || strings.ContainsAny(request.Credentials.AccessToken, "\r\n\x00") {
			return accountsend.Result{Outcome: accountsend.OutcomeRejected, Code: "oauth_unavailable"}, nil
		}
		authCommand = "AUTH XOAUTH2 " + xoauth2(request.Account.MailboxAddress, request.Credentials.AccessToken)
	} else {
		if !strings.Contains(strings.ToUpper(capabilities), "AUTH PLAIN") {
			return accountsend.Result{Outcome: accountsend.OutcomeRejected, Code: "auth_plain_unavailable"}, nil
		}
		username := credentialUsername(endpoint, accounts.ValidationRequest{MailboxAddress: request.Account.MailboxAddress, Credentials: request.Credentials})
		if username == "" || request.Credentials.Password == "" || strings.ContainsAny(username+request.Credentials.Password, "\r\n\x00") {
			return accountsend.Result{Outcome: accountsend.OutcomeRejected, Code: "credentials_unavailable"}, nil
		}
		plain := base64.StdEncoding.EncodeToString([]byte("\x00" + username + "\x00" + request.Credentials.Password))
		authCommand = "AUTH PLAIN " + plain
	}
	if err := writeProtocolLine(connection, authCommand); err != nil {
		return accountsend.Result{}, err
	}
	if code, _ := readSMTPReply(reader); code != 235 {
		return smtpSubmissionReply(code, "authentication_rejected"), nil
	}
	if err := writeProtocolLine(connection, "MAIL FROM:<"+request.Submission.EnvelopeFrom+">"); err != nil {
		return accountsend.Result{}, err
	}
	if code, _ := readSMTPReply(reader); code != 250 {
		return smtpSubmissionReply(code, "sender_rejected"), nil
	}
	for _, recipient := range request.Submission.Recipients {
		if err := writeProtocolLine(connection, "RCPT TO:<"+recipient+">"); err != nil {
			return accountsend.Result{}, err
		}
		code, _ := readSMTPReply(reader)
		if code != 250 && code != 251 {
			_ = writeProtocolLine(connection, "RSET")
			_, _ = readSMTPReply(reader)
			return smtpSubmissionReply(code, "recipient_rejected"), nil
		}
	}
	if err := writeProtocolLine(connection, "DATA"); err != nil {
		return accountsend.Result{}, err
	}
	if code, _ := readSMTPReply(reader); code != 354 {
		return smtpSubmissionReply(code, "data_rejected"), nil
	}
	if err := writeSMTPData(connection, request.Submission.RawRFC822); err != nil {
		return accountsend.Result{Outcome: accountsend.OutcomeAmbiguous, Code: "data_write_ambiguous"}, nil
	}
	code, _ := readSMTPReply(reader)
	if code == 0 {
		return accountsend.Result{Outcome: accountsend.OutcomeAmbiguous, Code: "delivery_ambiguous"}, nil
	}
	result := smtpSubmissionReply(code, "delivery_rejected")
	if code == 250 {
		result = accountsend.Result{Outcome: accountsend.OutcomeSent}
	}
	_ = writeProtocolLine(connection, "QUIT")
	_, _ = readSMTPReply(reader)
	return result, nil
}

func smtpSubmissionReply(code int, rejectedCode string) accountsend.Result {
	if code >= 400 && code < 500 {
		return accountsend.Result{Outcome: accountsend.OutcomeRetry, Code: "provider_temporary_failure"}
	}
	if code >= 500 && code < 600 {
		return accountsend.Result{Outcome: accountsend.OutcomeRejected, Code: rejectedCode}
	}
	return accountsend.Result{Outcome: accountsend.OutcomeRetry, Code: "provider_protocol_failure"}
}

func validSubmissionSource(raw []byte, envelopeFrom string) bool {
	if len(raw) == 0 || len(raw) > accountsend.MaxRawBytes || !validCRLF(raw) {
		return false
	}
	message, err := mail.ReadMessage(bufio.NewReader(strings.NewReader(string(raw))))
	if err != nil || message.Header.Get("Message-ID") == "" || message.Header.Get("Bcc") != "" {
		return false
	}
	from, err := mail.ParseAddress(message.Header.Get("From"))
	return err == nil && strings.EqualFold(from.Address, envelopeFrom)
}

func validCRLF(raw []byte) bool {
	for index, value := range raw {
		if value == 0 || value == '\n' && (index == 0 || raw[index-1] != '\r') || value == '\r' && (index+1 == len(raw) || raw[index+1] != '\n') {
			return false
		}
	}
	for _, line := range strings.Split(string(raw), "\r\n") {
		if len(line) > 998 {
			return false
		}
	}
	return true
}

func writeSMTPData(connection interface{ Write([]byte) (int, error) }, raw []byte) error {
	lines := strings.Split(strings.TrimSuffix(string(raw), "\r\n"), "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, ".") {
			line = "." + line
		}
		if _, err := fmt.Fprintf(connection, "%s\r\n", line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(connection, ".\r\n")
	return err
}

var _ accountsend.Connector = (*SMTPConnector)(nil)
