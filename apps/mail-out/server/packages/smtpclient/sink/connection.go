package sink

import (
	"crypto/tls"
	"encoding/base64"
	"net"
	"net/textproto"
	"strings"
)

type connection struct {
	sink       *Sink
	network    net.Conn
	text       *textproto.Conn
	secure     bool
	authUser   string
	from       string
	hasSender  bool
	recipients []string
	open       bool
	commands   commands
	replies    replies
}

func newConnection(sink *Sink, network net.Conn) *connection {
	connection := &connection{sink: sink, network: network, text: textproto.NewConn(network), open: true}
	connection.wrapImplicitTls()
	return connection
}

func (connection *connection) wrapImplicitTls() {
	if !connection.sink.options.ImplicitTls {
		return
	}
	secured := tls.Server(connection.network, connection.sink.options.Tls)
	connection.network = secured
	connection.text = textproto.NewConn(secured)
	connection.secure = true
}

func (connection *connection) run() {
	defer connection.network.Close()
	connection.reply(connection.replies.Ready())
	for connection.open {
		verb, argument := connection.next()
		connection.handler(verb)(argument)
	}
}

func (connection *connection) next() (string, string) {
	line, err := connection.text.ReadLine()
	if err != nil {
		return connection.commands.Gone(), ""
	}
	verb, argument, _ := strings.Cut(line, " ")
	return strings.ToUpper(verb), argument
}

func (connection *connection) handler(verb string) func(string) {
	handler, found := connection.handlers()[verb]
	if !found {
		return connection.unknown
	}
	return handler
}

func (connection *connection) handlers() map[string]func(string) {
	return map[string]func(string){
		connection.commands.Gone():     connection.gone,
		connection.commands.Ehlo():     connection.hello,
		connection.commands.Helo():     connection.hello,
		connection.commands.StartTls(): connection.startTls,
		connection.commands.Auth():     connection.auth,
		connection.commands.Mail():     connection.mail,
		connection.commands.Rcpt():     connection.recipient,
		connection.commands.Data():     connection.data,
		connection.commands.Rset():     connection.reset,
		connection.commands.Noop():     connection.noop,
		connection.commands.Quit():     connection.quit,
	}
}

func (connection *connection) reply(answer reply) {
	connection.text.PrintfLine("%d %s", answer.code, answer.text)
}

func (connection *connection) gone(string) {
	connection.open = false
}

func (connection *connection) unknown(string) {
	connection.reply(connection.replies.Unknown())
}

func (connection *connection) noop(string) {
	connection.reply(connection.replies.Okay())
}

func (connection *connection) quit(string) {
	connection.reply(connection.replies.Closing())
	connection.open = false
}

func (connection *connection) reset(string) {
	connection.clearTransaction()
	connection.reply(connection.replies.Okay())
}

func (connection *connection) clearTransaction() {
	connection.from = ""
	connection.hasSender = false
	connection.recipients = nil
}

func (connection *connection) hello(string) {
	connection.clearTransaction()
	lines := append([]string{"sink"}, connection.startTlsExtension()...)
	lines = append(lines, connection.authExtension()...)
	connection.writeLines(connection.replies.Okay().code, lines)
}

func (connection *connection) writeLines(code int, lines []string) {
	last := len(lines) - 1
	for index, line := range lines {
		connection.text.PrintfLine("%d%s%s", code, connection.separator(index == last), line)
	}
}

func (connection *connection) separator(last bool) string {
	if last {
		return " "
	}
	return "-"
}

func (connection *connection) startTlsExtension() []string {
	if !connection.tlsAvailable() {
		return nil
	}
	return []string{connection.commands.StartTls()}
}

func (connection *connection) authExtension() []string {
	if !connection.authAvailable() {
		return nil
	}
	return []string{connection.commands.Auth() + " PLAIN"}
}

func (connection *connection) tlsAvailable() bool {
	return connection.sink.options.Tls != nil && !connection.secure
}

func (connection *connection) authAvailable() bool {
	return connection.sink.options.Username != "" && connection.secure
}

func (connection *connection) startTls(string) {
	if !connection.tlsAvailable() {
		connection.reply(connection.replies.TlsUnavailable())
		return
	}
	connection.reply(connection.replies.TlsReady())
	connection.upgrade(tls.Server(connection.network, connection.sink.options.Tls))
}

func (connection *connection) upgrade(secured *tls.Conn) {
	connection.open = secured.Handshake() == nil
	connection.network = secured
	connection.text = textproto.NewConn(secured)
	connection.secure = true
	connection.clearTransaction()
}

func (connection *connection) auth(argument string) {
	if !connection.authAvailable() {
		connection.reply(connection.replies.AuthUnavailable())
		return
	}
	connection.reply(connection.login(connection.credentials(argument)))
}

func (connection *connection) credentials(argument string) (string, string) {
	_, encoded, _ := strings.Cut(argument, " ")
	decoded, _ := base64.StdEncoding.DecodeString(encoded)
	parts := append(strings.SplitN(string(decoded), "\x00", 3), "", "", "")
	return parts[1], parts[2]
}

func (connection *connection) login(username string, password string) reply {
	options := connection.sink.options
	if username != options.Username || password != options.Password {
		return connection.replies.AuthFailed()
	}
	connection.authUser = username
	return connection.replies.AuthOkay()
}

func (connection *connection) mail(argument string) {
	if !connection.authorized() {
		connection.reply(connection.replies.AuthRequired())
		return
	}
	connection.clearTransaction()
	connection.from = connection.address(argument)
	connection.hasSender = true
	connection.reply(connection.replies.Okay())
}

func (connection *connection) authorized() bool {
	return connection.sink.options.Username == "" || connection.authUser != ""
}

func (connection *connection) address(argument string) string {
	_, rest, _ := strings.Cut(argument, "<")
	value, _, _ := strings.Cut(rest, ">")
	return value
}

func (connection *connection) recipient(argument string) {
	if !connection.hasSender {
		connection.reply(connection.replies.BadSequence())
		return
	}
	connection.reply(connection.acceptRecipient(connection.address(argument)))
}

func (connection *connection) acceptRecipient(address string) reply {
	rejection := connection.rejection(address)
	if rejection.code != 0 {
		return rejection
	}
	connection.recipients = append(connection.recipients, address)
	return connection.replies.Okay()
}

func (connection *connection) rejection(address string) reply {
	reject := connection.sink.options.RejectRecipient
	if reject == nil {
		return reply{}
	}
	code, text := reject(address)
	return reply{code: code, text: text}
}

func (connection *connection) data(string) {
	if len(connection.recipients) == 0 {
		connection.reply(connection.replies.BadSequence())
		return
	}
	connection.reply(connection.replies.StartData())
	connection.finishData(connection.readData())
}

func (connection *connection) readData() ([]byte, error) {
	var lines []string
	line, err := connection.text.ReadLine()
	for err == nil && line != "." {
		lines = append(lines, strings.TrimPrefix(line, "."))
		line, err = connection.text.ReadLine()
	}
	return []byte(strings.Join(append(lines, ""), "\r\n")), err
}

func (connection *connection) finishData(body []byte, err error) {
	if err != nil {
		connection.open = false
		return
	}
	connection.reply(connection.accept(body))
}

func (connection *connection) accept(body []byte) reply {
	if connection.tooLarge(body) {
		return connection.replies.TooLarge()
	}
	connection.sink.store(Received{From: connection.from, Recipients: connection.recipients, Data: body, Tls: connection.secure, AuthUser: connection.authUser})
	connection.clearTransaction()
	return connection.replies.Queued()
}

func (connection *connection) tooLarge(body []byte) bool {
	maximum := connection.sink.options.MaxSize
	return maximum > 0 && len(body) > maximum
}
