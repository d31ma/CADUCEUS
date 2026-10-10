package smtpd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxCommandLength = 4 * 1024
	readerSize       = 64 * 1024
)

var (
	httpRequestPattern = regexp.MustCompile(`(?i)^(OPTIONS|GET|HEAD|POST|PUT|DELETE|TRACE|CONNECT) /.* HTTP/\d\.\d$`)
	hostnameLabel      = regexp.MustCompile(`(?i)^[a-z0-9_]([a-z0-9_-]*[a-z0-9_])?$`)
	errLineTooLong     = errors.New("Command line too long")
)

type handler func(conn *connection, line string)

var handlers map[string]handler

func init() {
	handlers = map[string]handler{
		"EHLO":     (*connection).handleEHLO,
		"HELO":     (*connection).handleHELO,
		"QUIT":     (*connection).handleQUIT,
		"NOOP":     (*connection).handleNOOP,
		"RSET":     (*connection).handleRSET,
		"HELP":     (*connection).handleHELP,
		"VRFY":     (*connection).handleVRFY,
		"MAIL":     (*connection).handleMAIL,
		"RCPT":     (*connection).handleRCPT,
		"DATA":     (*connection).handleDATA,
		"STARTTLS": (*connection).handleSTARTTLS,
		"AUTH":     (*connection).handleAUTH,
		"WIZ":      (*connection).handleWIZ,
		"SHELL":    (*connection).handleSHELL,
		"KILL":     (*connection).handleKILL,
	}
}

type connection struct {
	server *Server

	mu     sync.Mutex
	conn   net.Conn
	closed bool

	reader  *bufio.Reader
	session Session
	ctx     context.Context
	cancel  context.CancelFunc

	next           handler
	unrecognized   int
	unauthenticate int
	isWizard       bool
}

func newConnection(server *Server, raw net.Conn, sequence uint64) *connection {
	ctx, cancel := context.WithCancel(context.Background())
	conn := &connection{server: server, conn: raw, ctx: ctx, cancel: cancel}
	conn.session.ID = connectionID(sequence)
	conn.session.RemoteAddress, conn.session.RemotePort = splitAddress(raw.RemoteAddr())
	conn.session.LocalAddress, conn.session.LocalPort = splitAddress(raw.LocalAddr())
	return conn
}

func splitAddress(address net.Addr) (string, int) {
	if address == nil {
		return "", 0
	}
	host, port, err := net.SplitHostPort(address.String())
	if err != nil {
		return address.String(), 0
	}
	host = strings.TrimPrefix(host, "::ffff:")
	number, _ := strconv.Atoi(port)
	return host, number
}

func (conn *connection) serve() {
	defer conn.close()
	options := conn.server.options
	if options.Secure {
		if err := conn.handshake(); err != nil {
			conn.server.reportError(err)
			return
		}
	}
	conn.reader = bufio.NewReaderSize(conn.current(), readerSize)

	// The early-talker guard: anything the client sends during the ready delay
	// or the reverse lookup is refused before the greeting.
	early := conn.waitForSilence(options.ReadyDelay)
	if conn.isClosed() {
		return
	}
	conn.session.ClientHostname = conn.resolveHostname()
	conn.resetSession()

	if early || conn.waitForSilence(time.Millisecond) {
		conn.send(421, options.Name+" You talk too soon")
		return
	}
	greeting := options.Name + " ESMTP"
	if options.Banner != "" {
		greeting += " " + options.Banner
	}
	conn.send(220, greeting)

	for !conn.isClosed() {
		line, err := conn.readLine()
		if err != nil {
			var timeout net.Error
			switch {
			case errors.Is(err, errLineTooLong):
				conn.send(421, "Error: "+err.Error())
			case errors.As(err, &timeout) && timeout.Timeout():
				conn.send(421, "Timeout - closing connection")
			}
			return
		}
		conn.onCommand(line)
	}
}

func (conn *connection) current() net.Conn {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.conn
}

func (conn *connection) isClosed() bool {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.closed
}

func (conn *connection) close() {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closed {
		return
	}
	conn.closed = true
	conn.cancel()
	_ = conn.conn.Close()
}

func (conn *connection) terminate() {
	conn.send(421, "Server shutting down")
}

func (conn *connection) handshake() error {
	conn.mu.Lock()
	secure := tls.Server(conn.conn, conn.server.options.TLSConfig)
	conn.conn = secure
	conn.mu.Unlock()
	_ = secure.SetDeadline(time.Now().Add(conn.server.options.SocketTimeout))
	if err := secure.HandshakeContext(conn.ctx); err != nil {
		return fmt.Errorf("Failed to establish TLS session: %w", err)
	}
	_ = secure.SetDeadline(time.Time{})
	state := secure.ConnectionState()
	conn.session.Secure = true
	conn.session.ServerName = state.ServerName
	return nil
}

func (conn *connection) resolveHostname() string {
	hostname := ""
	if !conn.server.options.DisableReverseLookup && conn.session.RemoteAddress != "" {
		ctx, cancel := context.WithTimeout(conn.ctx, 1500*time.Millisecond)
		names, err := conn.server.options.LookupAddr(ctx, conn.session.RemoteAddress)
		cancel()
		if err == nil && len(names) > 0 && validHostname(names[0]) {
			hostname = names[0]
		}
	}
	hostname = strings.TrimSuffix(strings.ToLower(hostname), ".")
	if hostname == "" {
		return "[" + conn.session.RemoteAddress + "]"
	}
	return hostname
}

func validHostname(hostname string) bool {
	if hostname == "" || len(hostname) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(hostname, "."), ".") {
		if label == "" || len(label) > 63 || !hostnameLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// waitForSilence waits up to window and reports whether the client sent
// anything in it.
func (conn *connection) waitForSilence(window time.Duration) bool {
	raw := conn.current()
	_ = raw.SetReadDeadline(time.Now().Add(window))
	_, err := conn.reader.Peek(1)
	_ = raw.SetReadDeadline(time.Time{})
	if err == nil {
		return true
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		conn.close()
	}
	return false
}

// readLine answers the next command line without its CRLF. Like smtp-stream,
// it fails only when the data received so far holds more than 4 KiB with no
// line end; a long line that arrives whole is still processed.
func (conn *connection) readLine() (string, error) {
	raw := conn.current()
	_ = raw.SetReadDeadline(time.Now().Add(conn.server.options.SocketTimeout))
	for {
		buffered := conn.reader.Buffered()
		if buffered > 0 {
			window, _ := conn.reader.Peek(buffered)
			if index := bytes.IndexByte(window, '\n'); index >= 0 {
				line := make([]byte, index)
				copy(line, window[:index])
				_, _ = conn.reader.Discard(index + 1)
				return string(bytes.TrimSuffix(line, []byte{'\r'})), nil
			}
			if buffered > maxCommandLength {
				_, _ = conn.reader.Discard(buffered)
				return "", errLineTooLong
			}
		}
		if _, err := conn.reader.Peek(buffered + 1); err != nil {
			return "", err
		}
	}
}

func sanitize(line string) string {
	var builder strings.Builder
	for _, character := range line {
		if character <= 0x1f || (character >= 0x7f && character <= 0x9f) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

// send writes one reply. A 421 also closes the connection.
func (conn *connection) send(code int, lines ...string) {
	var payload string
	if len(lines) > 1 {
		parts := make([]string, len(lines))
		for index, line := range lines {
			separator := " "
			if index < len(lines)-1 {
				separator = "-"
			}
			parts[index] = strconv.Itoa(code) + separator + sanitize(line)
		}
		payload = strings.Join(parts, "\r\n")
	} else {
		payload = strconv.Itoa(code)
		if len(lines) == 1 && lines[0] != "" {
			payload += " " + sanitize(lines[0])
		}
		if code == 334 && payload == "334" {
			payload += " "
		}
	}
	conn.mu.Lock()
	if !conn.closed {
		_ = conn.conn.SetWriteDeadline(time.Now().Add(conn.server.options.SocketTimeout))
		_, _ = io.WriteString(conn.conn, payload+"\r\n")
	}
	conn.mu.Unlock()
	if code == 421 {
		conn.close()
	}
}

func (conn *connection) resetSession() {
	conn.session.Envelope = Envelope{BodyType: "7bit"}
}

func (conn *connection) onCommand(line string) {
	server := conn.server
	options := server.options
	commandName := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
	if server.isClosing() {
		conn.send(421, "Server shutting down")
		return
	}
	if httpRequestPattern.MatchString(line) {
		conn.send(421, "HTTP requests not allowed")
		return
	}
	var handle handler
	if conn.next != nil {
		handle = conn.next
		conn.next = nil
	} else {
		if commandName == "HELO" || commandName == "EHLO" {
			conn.session.OpeningCommand = commandName
		}
		if server.supported(commandName) {
			handle = handlers[commandName]
		}
	}
	if handle == nil {
		conn.unrecognized++
		if conn.unrecognized >= 10 {
			conn.send(421, "Error: too many unrecognized commands")
			return
		}
		conn.send(500, "Error: command not recognized")
		return
	}
	authEnforced := conn.session.User == "" && server.supported("AUTH") && !options.AuthOptional
	if authEnforced && commandName != "AUTH" {
		conn.unauthenticate++
		if conn.unauthenticate >= options.MaxUnauthenticatedCommands {
			conn.send(421, "Error: too many unauthenticated commands")
			return
		}
	}
	transaction := commandName == "MAIL" || commandName == "RCPT" || commandName == "DATA"
	if conn.session.HostNameAppearsAs == "" && (transaction || commandName == "AUTH") {
		conn.send(503, "Error: send HELO/EHLO first")
		return
	}
	if authEnforced && transaction {
		conn.send(530, "Error: authentication Required")
		return
	}
	handle(conn, line)
}

func fields(line string) []string {
	return whitespacePattern.Split(strings.TrimSpace(line), -1)
}

func (conn *connection) handleEHLO(line string) {
	parts := fields(line)
	if len(parts) != 2 {
		conn.send(501, "Error: syntax: EHLO hostname")
		return
	}
	options := conn.server.options
	conn.session.HostNameAppearsAs = strings.ToLower(parts[1])
	features := []string{"PIPELINING", "8BITMIME", "SMTPUTF8"}
	if len(options.AuthMethods) > 0 && conn.server.supported("AUTH") && conn.session.User == "" {
		features = append(features, "AUTH "+strings.Join(options.AuthMethods, " "))
	}
	if !conn.session.Secure && conn.server.supported("STARTTLS") && !options.HideSTARTTLS {
		features = append(features, "STARTTLS")
	}
	if options.Size > 0 {
		features = append(features, "SIZE "+strconv.FormatInt(options.Size, 10))
	}
	conn.resetSession()
	conn.send(250, append([]string{options.Name + " Nice to meet you, " + conn.session.ClientHostname}, features...)...)
}

func (conn *connection) handleHELO(line string) {
	parts := fields(line)
	if len(parts) != 2 {
		conn.send(501, "Error: Syntax: HELO hostname")
		return
	}
	conn.session.HostNameAppearsAs = strings.ToLower(parts[1])
	conn.resetSession()
	conn.send(250, conn.server.options.Name+" Nice to meet you, "+conn.session.ClientHostname)
}

func (conn *connection) handleQUIT(string) {
	conn.send(221, "Bye")
	conn.close()
}

func (conn *connection) handleNOOP(string) { conn.send(250, "OK") }

func (conn *connection) handleRSET(string) {
	conn.resetSession()
	conn.send(250, "Flushed")
}

func (conn *connection) handleHELP(string) {
	conn.send(214, "See https://tools.ietf.org/html/rfc5321 for details")
}

func (conn *connection) handleVRFY(string) {
	conn.send(252, "Try to send something. No promises though")
}

func (conn *connection) handleWIZ(line string) {
	parts := fields(line)
	if len(parts) < 2 || parts[1] == "" {
		conn.send(500, "You are no wizard!")
		return
	}
	conn.isWizard = true
	conn.send(200, "Please pass, oh mighty wizard")
}

func (conn *connection) handleSHELL(string) {
	if !conn.isWizard {
		conn.send(500, "Mere mortals must not mutter that mantra")
		return
	}
	conn.send(500, "Error: Invoking shell is not allowed. This incident will be reported.")
}

func (conn *connection) handleKILL(string) { conn.send(500, "Can not kill Mom") }

func replyFor(err error, fallback int) (int, string) {
	var reply *Reply
	if errors.As(err, &reply) {
		code := reply.Code
		if code == 0 {
			code = fallback
		}
		return code, reply.Message
	}
	return fallback, err.Error()
}

func (conn *connection) handleMAIL(line string) {
	options := conn.server.options
	parsed, ok := parseAddressCommand("mail from", line)
	if !ok {
		conn.send(501, "Error: Bad sender address syntax")
		return
	}
	if conn.session.Envelope.MailFrom != nil {
		conn.send(503, "Error: nested MAIL command")
		return
	}
	if size, ok := parsed.Args["SIZE"]; ok && options.Size > 0 {
		// JavaScript Number(): a bare SIZE keyword is true, which is 1.
		declared, err := 1.0, error(nil)
		if !size.Flag {
			declared, err = strconv.ParseFloat(strings.TrimSpace(size.Value), 64)
		}
		if err == nil && declared > float64(options.Size) {
			conn.send(552, "Error: message exceeds fixed maximum message size "+strconv.FormatInt(options.Size, 10))
			return
		}
	}
	if body, ok := parsed.Args["BODY"]; ok {
		// smtp-server crashed on a bare BODY keyword; refuse it instead.
		kind := strings.ToLower(body.Value)
		if body.Flag || (kind != "7bit" && kind != "8bitmime") {
			conn.send(501, "Invalid BODY parameter value. Must be 7BIT or 8BITMIME")
			return
		}
	}
	if utf8, ok := parsed.Args["SMTPUTF8"]; ok && !utf8.Flag {
		conn.send(501, "Invalid SMTPUTF8 parameter. This flag does not accept a value")
		return
	}
	if requireTLS, ok := parsed.Args["REQUIRETLS"]; ok {
		if !conn.session.Secure {
			conn.send(530, "REQUIRETLS not permitted on non-TLS connections")
			return
		}
		if !requireTLS.Flag {
			conn.send(501, "Invalid REQUIRETLS parameter. This flag does not accept a value")
			return
		}
	}
	if body, ok := parsed.Args["BODY"]; ok && !body.Flag {
		conn.session.Envelope.BodyType = strings.ToLower(body.Value)
	}
	if utf8, ok := parsed.Args["SMTPUTF8"]; ok && utf8.Flag {
		conn.session.Envelope.SMTPUTF8 = true
	}
	if options.OnMailFrom != nil {
		if err := options.OnMailFrom(conn.ctx, parsed, &conn.session); err != nil {
			conn.send(replyFor(err, 550))
			return
		}
	}
	conn.session.Envelope.MailFrom = &parsed
	conn.send(250, "Accepted")
}

func (conn *connection) handleRCPT(line string) {
	parsed, ok := parseAddressCommand("rcpt to", line)
	if !ok || parsed.Address == "" {
		conn.send(501, "Error: Bad recipient address syntax")
		return
	}
	if conn.session.Envelope.MailFrom == nil {
		conn.send(503, "Error: need MAIL command")
		return
	}
	if onRcptTo := conn.server.options.OnRcptTo; onRcptTo != nil {
		if err := onRcptTo(conn.ctx, parsed, &conn.session); err != nil {
			conn.send(replyFor(err, 550))
			return
		}
	}
	replaced := false
	for index, existing := range conn.session.Envelope.RcptTo {
		if strings.EqualFold(existing.Address, parsed.Address) {
			conn.session.Envelope.RcptTo[index] = parsed
			replaced = true
			break
		}
	}
	if !replaced {
		conn.session.Envelope.RcptTo = append(conn.session.Envelope.RcptTo, parsed)
	}
	conn.send(250, "Accepted")
}

func (conn *connection) handleDATA(string) {
	if len(conn.session.Envelope.RcptTo) == 0 {
		conn.send(503, "Error: need RCPT command")
		return
	}
	conn.send(354, "End data with <CR><LF>.<CR><LF>")
	raw, exceeded, err := conn.readData()
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			conn.send(421, "Timeout - closing connection")
		} else {
			conn.close()
		}
		return
	}
	var result error
	if onData := conn.server.options.OnData; onData != nil {
		result = onData(conn.ctx, raw, exceeded, &conn.session)
	}
	if result != nil {
		conn.send(replyFor(result, 450))
	} else {
		conn.send(250, "OK: message queued")
	}
	conn.unrecognized = 0
	conn.resetSession()
}

// readData reads until the CRLF.CRLF terminator as smtp-stream's data mode
// does: the terminator must follow a CRLF (or open the data), a line that
// starts with ".." loses one dot, and CRLF line ends are kept.
func (conn *connection) readData() ([]byte, bool, error) {
	limit := conn.server.options.Size
	var buffer bytes.Buffer
	var total int64
	atLineStart, previousCRLF, atStart := true, true, true
	raw := conn.current()
	for {
		_ = raw.SetReadDeadline(time.Now().Add(conn.server.options.SocketTimeout))
		chunk, err := conn.reader.ReadSlice('\n')
		if err != nil && !errors.Is(err, bufio.ErrBufferFull) {
			return nil, false, err
		}
		segment := chunk
		if atLineStart {
			if (atStart || previousCRLF) && bytes.Equal(segment, []byte(".\r\n")) {
				return buffer.Bytes(), limit > 0 && total > limit, nil
			}
			if bytes.HasPrefix(segment, []byte("..")) {
				segment = segment[1:]
			}
		}
		if len(segment) > 0 {
			atStart = false
		}
		total += int64(len(segment))
		if limit <= 0 || int64(buffer.Len()) < limit {
			room := len(segment)
			if limit > 0 && int64(buffer.Len()+room) > limit {
				room = int(limit) - buffer.Len()
			}
			buffer.Write(segment[:room])
		}
		complete := err == nil
		if complete {
			previousCRLF = bytes.HasSuffix(chunk, []byte("\r\n"))
		}
		atLineStart = complete
	}
}

func (conn *connection) handleSTARTTLS(line string) {
	if len(fields(line)) > 1 {
		conn.send(501, "Error: syntax: STARTTLS")
		return
	}
	if conn.session.Secure {
		conn.send(503, "Error: TLS already active")
		return
	}
	conn.send(220, "Ready to start TLS")
	// Drop any plaintext pipelined after STARTTLS (RFC 3207 section 4.2).
	if buffered := conn.reader.Buffered(); buffered > 0 {
		_, _ = conn.reader.Discard(buffered)
	}
	if err := conn.handshake(); err != nil {
		conn.server.reportError(err)
		conn.close()
		return
	}
	conn.reader = bufio.NewReaderSize(conn.current(), readerSize)
	conn.session.User = ""
	conn.session.HostNameAppearsAs = ""
	conn.session.OpeningCommand = ""
	conn.resetSession()
}

func (conn *connection) handleAUTH(line string) {
	options := conn.server.options
	args := fields(line)[1:]
	method := ""
	if len(args) > 0 {
		method = strings.ToUpper(args[0])
		args = args[1:]
	}
	if !conn.session.Secure && conn.server.supported("STARTTLS") && !options.HideSTARTTLS && !options.AllowInsecureAuth {
		conn.send(538, "Error: Must issue a STARTTLS command first")
		return
	}
	if conn.session.User != "" {
		conn.send(503, "Error: No identity changes permitted")
		return
	}
	if !conn.server.authMethod(method) || (method != "PLAIN" && method != "LOGIN") {
		conn.send(504, "Error: Unrecognized authentication type")
		return
	}
	if method == "PLAIN" {
		if len(args) > 1 {
			conn.send(501, "Error: syntax: AUTH PLAIN token")
			return
		}
		if len(args) == 0 {
			conn.next = func(conn *connection, token string) { conn.plainToken(true, token) }
			conn.send(334)
			return
		}
		conn.plainToken(false, args[0])
		return
	}
	if len(args) > 1 {
		conn.send(501, "Error: syntax: AUTH LOGIN")
		return
	}
	if len(args) == 0 {
		conn.next = func(conn *connection, username string) { conn.loginUsername(true, username) }
		conn.send(334, "VXNlcm5hbWU6")
		return
	}
	conn.loginUsername(false, args[0])
}

// decodeBase64 is as lenient as Node's Buffer.from(value, 'base64').
func decodeBase64(value string) string {
	var cleaned strings.Builder
	for _, character := range value {
		switch {
		case character >= 'A' && character <= 'Z', character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			cleaned.WriteRune(character)
		case character == '+' || character == '-':
			cleaned.WriteByte('+')
		case character == '/' || character == '_':
			cleaned.WriteByte('/')
		case character == '=':
			goto done
		}
	}
done:
	text := cleaned.String()
	if len(text)%4 == 1 {
		text = text[:len(text)-1]
	}
	decoded, _ := base64.RawStdEncoding.DecodeString(text)
	return string(decoded)
}

func (conn *connection) plainToken(canAbort bool, token string) {
	token = strings.TrimSpace(token)
	if canAbort && token == "*" {
		conn.send(501, "Authentication aborted")
		return
	}
	data := strings.Split(decodeBase64(token), "\x00")
	if len(data) != 3 {
		conn.send(500, "Error: invalid userdata")
		return
	}
	authzid, authcid, password := data[0], data[1], data[2]
	username := authcid
	if username == "" {
		username = authzid
	}
	if hasControl(username) || hasControl(authzid) {
		conn.send(500, "Error: invalid userdata")
		return
	}
	conn.authenticate(Auth{Method: "PLAIN", Username: username, Authcid: authcid, Authzid: authzid, Password: password})
}

func (conn *connection) loginUsername(canAbort bool, username string) {
	username = strings.TrimSpace(username)
	if canAbort && username == "*" {
		conn.send(501, "Authentication aborted")
		return
	}
	username = decodeBase64(username)
	if username == "" {
		conn.send(500, "Error: missing username")
		return
	}
	if hasControl(username) {
		conn.send(500, "Error: invalid userdata")
		return
	}
	conn.next = func(conn *connection, password string) {
		password = strings.TrimSpace(password)
		if password == "*" {
			conn.send(501, "Authentication aborted")
			return
		}
		conn.authenticate(Auth{Method: "LOGIN", Username: username, Password: decodeBase64(password)})
	}
	conn.send(334, "UGFzc3dvcmQ6")
}

func (conn *connection) authenticate(auth Auth) {
	onAuth := conn.server.options.OnAuth
	if onAuth == nil {
		conn.send(535, "Authentication not implemented")
		return
	}
	user, err := onAuth(conn.ctx, auth, &conn.session)
	if err != nil {
		conn.send(replyFor(err, 535))
		return
	}
	if user == "" {
		conn.send(535, "Error: Authentication credentials invalid")
		return
	}
	conn.session.User = user
	conn.send(235, "Authentication successful")
}
