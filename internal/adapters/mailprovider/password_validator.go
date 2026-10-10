// Package mailprovider contains bounded external mail protocol adapters.
package mailprovider

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
)

const (
	maximumProtocolLine = 64 << 10
	maximumReplyLines   = 100
)

var ErrUnsafeEndpoint = errors.New("unsafe mail provider endpoint")

type IPResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type PasswordValidator struct {
	timeout         time.Duration
	dialer          net.Dialer
	resolver        IPResolver
	privateNetworks []*net.IPNet
}

func NewPasswordValidator(timeout time.Duration, privateCIDRs []string) (*PasswordValidator, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	networks, err := parsePrivateNetworks(privateCIDRs)
	if err != nil {
		return nil, err
	}
	return &PasswordValidator{
		timeout: timeout, dialer: net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second},
		resolver: net.DefaultResolver, privateNetworks: networks,
	}, nil
}

func (validator *PasswordValidator) Validate(ctx context.Context, request accounts.ValidationRequest) (accounts.Capabilities, error) {
	if request.IMAP == nil || request.SMTP == nil || !validCredential(request) {
		return accounts.Capabilities{}, accounts.ErrInvalid
	}
	if err := validator.validateIMAP(ctx, request); err != nil {
		return accounts.Capabilities{}, err
	}
	if err := validator.validateSMTP(ctx, request); err != nil {
		return accounts.Capabilities{}, err
	}
	return accounts.Capabilities{MailRead: true, MailWrite: true, Send: true, Labels: request.Provider == accounts.ProviderGmail, SharedMailbox: request.Provider == accounts.ProviderMicrosoft && request.SharedMailboxAddress != ""}, nil
}

func validCredential(request accounts.ValidationRequest) bool {
	if oauthMailProvider(request.Provider) {
		return request.MailboxAddress != "" && request.AuthIdentity != "" && request.Credentials.AccessToken != "" &&
			len(request.Credentials.AccessToken) <= 32768 && !strings.ContainsAny(request.Credentials.AccessToken, "\r\n\x00")
	}
	if request.Provider != accounts.ProviderIMAP {
		return false
	}
	username := credentialUsername(request.IMAP, request)
	return username != "" && request.Credentials.Password != "" && len(username) <= 1024 && len(request.Credentials.Password) <= 4096 &&
		!strings.ContainsAny(username, "\r\n\x00") && !strings.ContainsAny(request.Credentials.Password, "\r\n\x00")
}

func credentialUsername(endpoint *accounts.Endpoint, request accounts.ValidationRequest) string {
	if endpoint != nil && strings.TrimSpace(endpoint.Username) != "" {
		return strings.TrimSpace(endpoint.Username)
	}
	if strings.TrimSpace(request.Credentials.Username) != "" {
		return strings.TrimSpace(request.Credentials.Username)
	}
	return request.MailboxAddress
}

func (validator *PasswordValidator) validateIMAP(ctx context.Context, request accounts.ValidationRequest) error {
	connection, err := validator.connect(ctx, request.IMAP)
	if err != nil {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationConnection)
	}
	defer connection.Close()
	reader := protocolReader(connection)
	if !imapGreeting(reader) {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationProtocol)
	}
	if request.IMAP.Security == accounts.SecurityStartTLS {
		connection, reader, err = validator.startIMAPTLS(ctx, connection, reader, request.IMAP)
		if err != nil {
			return validationFailure(accounts.ValidationIMAP, accounts.ValidationConnection)
		}
	}
	authCommand := "a001 LOGIN " + imapQuote(credentialUsername(request.IMAP, request)) + " " + imapQuote(request.Credentials.Password)
	if oauthMailProvider(request.Provider) {
		authCommand = "a001 AUTHENTICATE XOAUTH2 " + xoauth2(request.MailboxAddress, request.Credentials.AccessToken)
	}
	if err := writeProtocolLine(connection, authCommand); err != nil {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationConnection)
	}
	status, ok := readIMAPTagged(reader, "a001")
	if !ok {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationProtocol)
	}
	if status != "OK" {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationAuthentication)
	}
	if err := writeProtocolLine(connection, `a002 SELECT "INBOX"`); err != nil {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationConnection)
	}
	status, detail, ok := readIMAPTaggedDetail(reader, "a002")
	if !ok {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationProtocol)
	}
	if status != "OK" {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationMailbox)
	}
	if request.Provider == accounts.ProviderMicrosoft && !strings.Contains(strings.ToUpper(detail), "READ-WRITE") {
		return validationFailure(accounts.ValidationIMAP, accounts.ValidationMailbox)
	}
	_ = writeProtocolLine(connection, "a003 LOGOUT")
	_, _ = readIMAPTagged(reader, "a003")
	return nil
}

func (validator *PasswordValidator) validateSMTP(ctx context.Context, request accounts.ValidationRequest) error {
	connection, err := validator.connect(ctx, request.SMTP)
	if err != nil {
		return validationFailure(accounts.ValidationSMTP, accounts.ValidationConnection)
	}
	defer connection.Close()
	reader := protocolReader(connection)
	if code, _ := readSMTPReply(reader); code != 220 {
		return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
	}
	capabilities, err := smtpHello(connection, reader)
	if err != nil {
		return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
	}
	if request.SMTP.Security == accounts.SecurityStartTLS {
		if !strings.Contains(strings.ToUpper(capabilities), "STARTTLS") {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
		}
		connection, reader, err = validator.startSMTPTLS(ctx, connection, reader, request.SMTP)
		if err != nil {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationConnection)
		}
		if _, err = smtpHello(connection, reader); err != nil {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
		}
	}
	authCommand := ""
	if oauthMailProvider(request.Provider) {
		if !strings.Contains(strings.ToUpper(capabilities), "XOAUTH2") {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
		}
		authCommand = "AUTH XOAUTH2 " + xoauth2(request.MailboxAddress, request.Credentials.AccessToken)
	} else {
		if !strings.Contains(strings.ToUpper(capabilities), "AUTH PLAIN") {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
		}
		username := credentialUsername(request.SMTP, request)
		plain := base64.StdEncoding.EncodeToString([]byte("\x00" + username + "\x00" + request.Credentials.Password))
		authCommand = "AUTH PLAIN " + plain
	}
	if err := writeProtocolLine(connection, authCommand); err != nil {
		return validationFailure(accounts.ValidationSMTP, accounts.ValidationConnection)
	}
	code, _ := readSMTPReply(reader)
	if code == 534 || code == 535 {
		return validationFailure(accounts.ValidationSMTP, accounts.ValidationAuthentication)
	}
	if code != 235 {
		return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
	}
	// Microsoft enforces delegated Send As separately from XOAUTH2. Exercise
	// only the envelope sender and immediately reset: no recipient or message
	// data is submitted, but a shared mailbox without sender authority fails
	// before CADUCEUS advertises the send capability.
	if oauthMailProvider(request.Provider) {
		if err := writeProtocolLine(connection, "MAIL FROM:<"+request.MailboxAddress+">"); err != nil {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationConnection)
		}
		if code, _ = readSMTPReply(reader); code != 250 {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationAuthentication)
		}
		if err := writeProtocolLine(connection, "RSET"); err != nil {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationConnection)
		}
		if code, _ = readSMTPReply(reader); code != 250 {
			return validationFailure(accounts.ValidationSMTP, accounts.ValidationProtocol)
		}
	}
	_ = writeProtocolLine(connection, "QUIT")
	_, _ = readSMTPReply(reader)
	return nil
}

func xoauth2(username, accessToken string) string {
	return base64.StdEncoding.EncodeToString([]byte("user=" + username + "\x01auth=Bearer " + accessToken + "\x01\x01"))
}

func oauthMailProvider(provider accounts.Provider) bool {
	return provider == accounts.ProviderMicrosoft || provider == accounts.ProviderGmail
}

func (validator *PasswordValidator) connect(ctx context.Context, endpoint *accounts.Endpoint) (net.Conn, error) {
	bounded, cancel := context.WithTimeout(ctx, validator.timeout)
	defer cancel()
	addresses, err := validator.resolveEndpoint(bounded, endpoint)
	if err != nil {
		return nil, err
	}
	var last error
	for _, ip := range addresses {
		address := net.JoinHostPort(ip.String(), strconv.Itoa(int(endpoint.Port)))
		connection, dialErr := validator.dialer.DialContext(bounded, "tcp", address)
		if dialErr != nil {
			last = dialErr
			continue
		}
		if endpoint.Security == accounts.SecurityTLS {
			secure := tls.Client(connection, tlsConfig(endpoint))
			if dialErr = secure.HandshakeContext(bounded); dialErr != nil {
				_ = connection.Close()
				last = dialErr
				continue
			}
			connection = secure
		}
		if dialErr = connection.SetDeadline(time.Now().Add(validator.timeout)); dialErr != nil {
			_ = connection.Close()
			last = dialErr
			continue
		}
		return connection, nil
	}
	if last == nil {
		last = ErrUnsafeEndpoint
	}
	return nil, last
}

func (validator *PasswordValidator) resolveEndpoint(ctx context.Context, endpoint *accounts.Endpoint) ([]net.IP, error) {
	if endpoint == nil || validator == nil || validator.resolver == nil {
		return nil, ErrUnsafeEndpoint
	}
	var resolved []net.IPAddr
	if literal := net.ParseIP(endpoint.Host); literal != nil {
		resolved = []net.IPAddr{{IP: literal}}
	} else {
		addresses, err := validator.resolver.LookupIPAddr(ctx, endpoint.Host)
		if err != nil {
			return nil, err
		}
		resolved = addresses
	}
	if len(resolved) == 0 {
		return nil, ErrUnsafeEndpoint
	}
	result := make([]net.IP, 0, len(resolved))
	seen := make(map[string]struct{}, len(resolved))
	for _, address := range resolved {
		if !validator.approvedAddress(address.IP) {
			return nil, ErrUnsafeEndpoint
		}
		canonical := address.IP.String()
		if _, duplicate := seen[canonical]; duplicate {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, address.IP)
	}
	return result, nil
}

func (validator *PasswordValidator) approvedAddress(ip net.IP) bool {
	if ip == nil || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return false
	}
	if ip.IsPrivate() || ip.IsLoopback() {
		for _, network := range validator.privateNetworks {
			if network.Contains(ip) {
				return true
			}
		}
		return false
	}
	return ip.IsGlobalUnicast()
}

func parsePrivateNetworks(values []string) ([]*net.IPNet, error) {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		ip, network, err := net.ParseCIDR(value)
		ones, bits := 0, 0
		if err == nil {
			ones, bits = network.Mask.Size()
		}
		if err != nil || value != network.String() || !ip.Equal(network.IP) ||
			!(network.IP.IsPrivate() || network.IP.IsLoopback()) || bits == 32 && ones < 8 || bits == 128 && ones < 7 {
			return nil, fmt.Errorf("invalid private provider CIDR %q", value)
		}
		result = append(result, network)
	}
	return result, nil
}

func (validator *PasswordValidator) startIMAPTLS(ctx context.Context, connection net.Conn, reader *bufio.Reader, endpoint *accounts.Endpoint) (net.Conn, *bufio.Reader, error) {
	if err := writeProtocolLine(connection, "a000 STARTTLS"); err != nil {
		return nil, nil, err
	}
	status, ok := readIMAPTagged(reader, "a000")
	if !ok || status != "OK" || reader.Buffered() != 0 {
		return nil, nil, errors.New("IMAP STARTTLS refused")
	}
	secure := tls.Client(connection, tlsConfig(endpoint))
	if err := secure.HandshakeContext(ctx); err != nil {
		return nil, nil, err
	}
	return secure, protocolReader(secure), nil
}

func (validator *PasswordValidator) startSMTPTLS(ctx context.Context, connection net.Conn, reader *bufio.Reader, endpoint *accounts.Endpoint) (net.Conn, *bufio.Reader, error) {
	if err := writeProtocolLine(connection, "STARTTLS"); err != nil {
		return nil, nil, err
	}
	if code, _ := readSMTPReply(reader); code != 220 || reader.Buffered() != 0 {
		return nil, nil, errors.New("SMTP STARTTLS refused")
	}
	secure := tls.Client(connection, tlsConfig(endpoint))
	if err := secure.HandshakeContext(ctx); err != nil {
		return nil, nil, err
	}
	return secure, protocolReader(secure), nil
}

func tlsConfig(endpoint *accounts.Endpoint) *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         endpoint.Host,
		InsecureSkipVerify: endpoint.AllowInsecureTLS, // Explicit compatibility setting; never selected by discovery.
	}
}

func smtpHello(connection net.Conn, reader *bufio.Reader) (string, error) {
	if err := writeProtocolLine(connection, "EHLO caduceus.local"); err != nil {
		return "", err
	}
	code, reply := readSMTPReply(reader)
	if code != 250 {
		return "", errors.New("SMTP EHLO refused")
	}
	return reply, nil
}

func imapGreeting(reader *bufio.Reader) bool {
	line, err := readProtocolLine(reader)
	return err == nil && strings.HasPrefix(strings.ToUpper(line), "* OK")
}

func readIMAPTagged(reader *bufio.Reader, tag string) (string, bool) {
	status, _, ok := readIMAPTaggedDetail(reader, tag)
	return status, ok
}

func readIMAPTaggedDetail(reader *bufio.Reader, tag string) (string, string, bool) {
	lines := make([]string, 0, 4)
	for count := 0; count < maximumReplyLines; count++ {
		line, err := readProtocolLine(reader)
		if err != nil {
			return "", "", false
		}
		lines = append(lines, line)
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(fields[0], tag) {
			return strings.ToUpper(fields[1]), strings.Join(lines, "\n"), true
		}
	}
	return "", "", false
}

func readSMTPReply(reader *bufio.Reader) (int, string) {
	lines := make([]string, 0, 4)
	code := 0
	for count := 0; count < maximumReplyLines; count++ {
		line, err := readProtocolLine(reader)
		if err != nil || len(line) < 3 {
			return 0, ""
		}
		parsed, err := strconv.Atoi(line[:3])
		if err != nil || code != 0 && parsed != code {
			return 0, ""
		}
		code = parsed
		lines = append(lines, line)
		if len(line) == 3 || line[3] != '-' {
			return code, strings.Join(lines, "\n")
		}
	}
	return 0, ""
}

func readProtocolLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return "", fmt.Errorf("read protocol line: %w", err)
	}
	if len(line) > maximumProtocolLine || !strings.HasSuffix(string(line), "\r\n") {
		return "", errors.New("invalid protocol line")
	}
	return strings.TrimSuffix(string(line), "\r\n"), nil
}

func protocolReader(connection net.Conn) *bufio.Reader {
	return bufio.NewReaderSize(connection, maximumProtocolLine+1)
}

func writeProtocolLine(connection net.Conn, line string) error {
	if len(line) > maximumProtocolLine || strings.ContainsAny(line, "\r\n") {
		return errors.New("invalid protocol command")
	}
	_, err := fmt.Fprintf(connection, "%s\r\n", line)
	return err
}

func imapQuote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func validationFailure(service accounts.ValidationService, code accounts.ValidationCode) error {
	return accounts.ValidationFailure{Service: service, Code: code}
}
