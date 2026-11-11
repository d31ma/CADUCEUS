package protocol

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"slices"
	"strconv"
	"time"

	"mesail.dev/edge/smtp/reception"
)

type Session struct {
	settings   Settings
	id         string
	connection net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	greeted    bool
	helo       string
	secure     bool
	upgrade    bool
	closing    bool
	errorCount int
	sender     string
	hasSender  bool
	recipients []reception.Recipient
	addresses  map[string]bool
}

func NewSession(settings Settings, connection net.Conn, id string) *Session {
	session := &Session{settings: settings, id: id, addresses: map[string]bool{}}
	session.attach(connection)
	return session
}

func (session *Session) Run(ctx context.Context) {
	session.send(Replies{}.Greeting(session.settings.Hostname))
	session.serve(ctx)
}

func (session *Session) serve(ctx context.Context) {
	for !session.closing {
		session.step(ctx)
	}
}

func (session *Session) step(ctx context.Context) {
	command, err := session.readCommand()
	reply := session.handlerForRead(err)(ctx, command)
	session.send(reply)
	session.countError(reply)
	session.stopAfterTooManyErrors()
	session.upgradeWhenAsked()
}

func (session *Session) attach(connection net.Conn) {
	session.connection = connection
	session.reader = bufio.NewReaderSize(connection, Limits{}.ReaderSize())
	session.writer = bufio.NewWriter(connection)
}

func (session *Session) readCommand() (Command, error) {
	session.connection.SetReadDeadline(time.Now().Add(Limits{}.CommandTimeout()))
	line, err := session.readLine()
	return Command{}.Parse(line), err
}

func (session *Session) readLine() (string, error) {
	line, more, err := session.reader.ReadLine()
	if err != nil {
		return "", err
	}
	return session.wholeLine(line, more)
}

func (session *Session) wholeLine(line []byte, more bool) (string, error) {
	return session.lineReaderFor(line, more)(line, more)
}

func (session *Session) lineReaderFor(line []byte, more bool) func([]byte, bool) (string, error) {
	if more || len(line)+len((Limits{}).LineEnding()) > (Limits{}).MaxLineLength() {
		return session.refuseLine
	}
	return session.acceptLine
}

func (session *Session) acceptLine(line []byte, _ bool) (string, error) {
	return string(line), nil
}

func (session *Session) refuseLine(_ []byte, more bool) (string, error) {
	return "", session.skipRestOfLine(more)
}

func (session *Session) skipRestOfLine(more bool) error {
	for more {
		_, next, err := session.reader.ReadLine()
		if err != nil {
			return err
		}
		more = next
	}
	return LineTooLong{}
}

func (session *Session) handlerForRead(err error) Handler {
	var timeout net.Error
	switch {
	case err == nil:
		return session.dispatch
	case errors.Is(err, LineTooLong{}):
		return session.lineTooLong
	case errors.As(err, &timeout) && timeout.Timeout():
		return session.timeOut
	default:
		return session.connectionLost
	}
}

func (session *Session) dispatch(ctx context.Context, command Command) Reply {
	return session.handlerFor(command.Verb)(ctx, command)
}

func (session *Session) handlerFor(verb string) Handler {
	switch verb {
	case "HELO":
		return session.guarded(session.hello, session.needsArgument)
	case "EHLO":
		return session.guarded(session.extendedHello, session.needsArgument)
	case "STARTTLS":
		return session.guarded(session.startTls, session.needsTlsOffer, session.needsPlainConnection, session.needsGreeting)
	case "MAIL":
		return session.guarded(session.mail, session.needsGreeting, session.needsNoSender)
	case "RCPT":
		return session.guarded(session.recipient, session.needsSender, session.needsRoomForRecipient)
	case "DATA":
		return session.guarded(session.data, session.needsRecipients)
	case "RSET":
		return session.reset
	case "NOOP":
		return session.noop
	case "QUIT":
		return session.quit
	case "VRFY":
		return session.verify
	case "HELP":
		return session.help
	case "AUTH":
		return session.notImplemented
	default:
		return session.unknown
	}
}

func (session *Session) guarded(handler Handler, guards ...Guard) Handler {
	return func(ctx context.Context, command Command) Reply {
		return session.checkThenRun(ctx, command, handler, guards)
	}
}

func (session *Session) checkThenRun(ctx context.Context, command Command, handler Handler, guards []Guard) Reply {
	for _, guard := range guards {
		refusal := guard(command)
		if !refusal.IsEmpty() {
			return refusal
		}
	}
	return handler(ctx, command)
}

func (session *Session) needsArgument(command Command) Reply {
	if command.Argument == "" {
		return Replies{}.SyntaxError()
	}
	return Reply{}
}

func (session *Session) needsGreeting(_ Command) Reply {
	if !session.greeted {
		return Replies{}.SendHelloFirst()
	}
	return Reply{}
}

func (session *Session) needsNoSender(_ Command) Reply {
	if session.hasSender {
		return Replies{}.SenderAlreadyGiven()
	}
	return Reply{}
}

func (session *Session) needsSender(_ Command) Reply {
	if !session.hasSender {
		return Replies{}.SendMailFirst()
	}
	return Reply{}
}

func (session *Session) needsRoomForRecipient(_ Command) Reply {
	if len(session.recipients) >= (Limits{}).MaxRecipients() {
		return Replies{}.TooManyRecipients()
	}
	return Reply{}
}

func (session *Session) needsRecipients(_ Command) Reply {
	if len(session.recipients) == 0 {
		return Replies{}.SendRecipientFirst()
	}
	return Reply{}
}

func (session *Session) needsTlsOffer(_ Command) Reply {
	if session.settings.Tls == nil {
		return Replies{}.NotImplemented()
	}
	return Reply{}
}

func (session *Session) needsPlainConnection(_ Command) Reply {
	if session.secure {
		return Replies{}.AlreadySecure()
	}
	return Reply{}
}

func (session *Session) hello(_ context.Context, command Command) Reply {
	session.greet(command.Argument)
	return Replies{}.Hello(session.settings.Hostname)
}

func (session *Session) extendedHello(_ context.Context, command Command) Reply {
	session.greet(command.Argument)
	return Replies{}.ExtendedHello(session.settings.Hostname, session.extensions())
}

func (session *Session) greet(helo string) {
	session.resetTransaction()
	session.greeted = true
	session.helo = helo
}

func (session *Session) extensions() []string {
	extensions := []string{"SIZE " + strconv.FormatInt(session.settings.MaxSize, 10), "8BITMIME", "PIPELINING", "ENHANCEDSTATUSCODES"}
	return append(extensions, session.tlsExtension()...)
}

func (session *Session) tlsExtension() []string {
	if session.settings.Tls == nil || session.secure {
		return nil
	}
	return []string{"STARTTLS"}
}

func (session *Session) startTls(_ context.Context, _ Command) Reply {
	session.upgrade = true
	return Replies{}.ReadyForTls()
}

func (session *Session) upgradeWhenAsked() {
	if session.upgrade {
		session.upgradeConnection()
	}
}

func (session *Session) upgradeConnection() {
	secured := tls.Server(session.connection, session.settings.Tls)
	secured.SetDeadline(time.Now().Add(Limits{}.CommandTimeout()))
	err := secured.Handshake()
	session.attach(secured)
	session.upgrade = false
	session.secure = true
	session.greeted = false
	session.resetTransaction()
	session.closeOnError(err)
}

func (session *Session) mail(_ context.Context, command Command) Reply {
	request, err := Paths{}.Mail(command.Argument)
	if err != nil {
		return Replies{}.SyntaxError()
	}
	return session.acceptSender(request)
}

func (session *Session) acceptSender(request MailRequest) Reply {
	if request.Size > session.settings.MaxSize {
		return Replies{}.TooLarge()
	}
	session.sender = request.Sender
	session.hasSender = true
	return Replies{}.SenderOk()
}

func (session *Session) recipient(ctx context.Context, command Command) Reply {
	address, err := Paths{}.Recipient(command.Argument)
	if err != nil {
		return Replies{}.SyntaxError()
	}
	return session.recipientStep(address)(ctx, address)
}

func (session *Session) recipientStep(address string) func(context.Context, string) Reply {
	if session.addresses[address] {
		return session.alreadyAdded
	}
	return session.lookUp
}

func (session *Session) alreadyAdded(_ context.Context, _ string) Reply {
	return Replies{}.RecipientOk()
}

func (session *Session) lookUp(ctx context.Context, address string) Reply {
	found, err := session.settings.Directory.Lookup(ctx, address)
	if err != nil {
		return Replies{}.forLookup(err)
	}
	session.recipients = append(session.recipients, reception.Recipient{Address: address, DomainId: found.DomainId})
	session.addresses[address] = true
	return Replies{}.RecipientOk()
}

func (session *Session) data(ctx context.Context, _ Command) Reply {
	session.send(Replies{}.StartData())
	content, err := session.readMessage()
	reply := session.dataHandlerFor(err)(ctx, content)
	session.resetTransaction()
	return reply
}

func (session *Session) readMessage() ([]byte, error) {
	session.connection.SetReadDeadline(time.Now().Add(Limits{}.DataTimeout()))
	dot := textproto.NewReader(session.reader).DotReader()
	content, err := io.ReadAll(io.LimitReader(dot, session.settings.MaxSize+1))
	if err != nil {
		return nil, err
	}
	return session.withinLimit(content, dot)
}

func (session *Session) withinLimit(content []byte, dot io.Reader) ([]byte, error) {
	if int64(len(content)) > session.settings.MaxSize {
		io.Copy(io.Discard, dot)
		return nil, MessageTooLarge{}
	}
	return bytes.ReplaceAll(content, []byte(Limits{}.BareLineEnding()), []byte(Limits{}.LineEnding())), nil
}

func (session *Session) dataHandlerFor(err error) func(context.Context, []byte) Reply {
	switch {
	case err == nil:
		return session.deliver
	case errors.Is(err, MessageTooLarge{}):
		return session.tooLarge
	default:
		return session.lostDuringData
	}
}

func (session *Session) deliver(ctx context.Context, content []byte) Reply {
	receipt, err := session.settings.Delivery.Deliver(ctx, session.transaction(content))
	reply := Replies{}.forDelivery(err)
	session.record(TransactionRecord{Size: len(content), Reply: reply, InboundIds: receipt.InboundIds, Failure: err})
	return reply
}

func (session *Session) tooLarge(_ context.Context, _ []byte) Reply {
	reply := Replies{}.TooLarge()
	session.record(TransactionRecord{Reply: reply, Failure: MessageTooLarge{}})
	return reply
}

func (session *Session) lostDuringData(_ context.Context, _ []byte) Reply {
	session.closing = true
	return Reply{}
}

func (session *Session) transaction(content []byte) reception.Transaction {
	return reception.Transaction{
		EnvelopeFrom:  session.sender,
		Recipients:    slices.Clone(session.recipients),
		Raw:           content,
		RemoteAddress: session.connection.RemoteAddr().String(),
		Helo:          session.helo,
		Tls:           session.secure,
		ReceivedAt:    time.Now(),
	}
}

func (session *Session) record(record TransactionRecord) {
	record.SessionId = session.id
	record.Remote = session.connection.RemoteAddr().String()
	record.Secure = session.secure
	record.Recipients = len(session.recipients)
	session.settings.Log.Print(record.Line())
}

func (session *Session) reset(_ context.Context, _ Command) Reply {
	session.resetTransaction()
	return Replies{}.Ok()
}

func (session *Session) resetTransaction() {
	session.sender = ""
	session.hasSender = false
	session.recipients = nil
	session.addresses = map[string]bool{}
}

func (session *Session) noop(_ context.Context, _ Command) Reply {
	return Replies{}.Ok()
}

func (session *Session) quit(_ context.Context, _ Command) Reply {
	session.closing = true
	return Replies{}.Closing(session.settings.Hostname)
}

func (session *Session) verify(_ context.Context, _ Command) Reply {
	return Replies{}.CannotVerify()
}

func (session *Session) help(_ context.Context, _ Command) Reply {
	return Replies{}.Help()
}

func (session *Session) notImplemented(_ context.Context, _ Command) Reply {
	return Replies{}.NotImplemented()
}

func (session *Session) unknown(_ context.Context, _ Command) Reply {
	return Replies{}.UnknownCommand()
}

func (session *Session) lineTooLong(_ context.Context, _ Command) Reply {
	return Replies{}.LineTooLong()
}

func (session *Session) timeOut(_ context.Context, _ Command) Reply {
	session.closing = true
	return Replies{}.TimedOut(session.settings.Hostname)
}

func (session *Session) connectionLost(_ context.Context, _ Command) Reply {
	session.closing = true
	return Reply{}
}

func (session *Session) send(reply Reply) {
	if !reply.IsEmpty() {
		session.write(reply)
	}
}

func (session *Session) write(reply Reply) {
	session.connection.SetWriteDeadline(time.Now().Add(Limits{}.CommandTimeout()))
	_, err := session.writer.WriteString(reply.Wire())
	session.closeOnError(errors.Join(err, session.writer.Flush()))
}

func (session *Session) closeOnError(err error) {
	if err != nil {
		session.closing = true
	}
}

func (session *Session) countError(reply Reply) {
	if reply.IsPermanentError() {
		session.errorCount++
	}
}

func (session *Session) stopAfterTooManyErrors() {
	if session.errorCount >= (Limits{}).MaxErrors() && !session.closing {
		session.write(Replies{}.TooManyErrors())
		session.closing = true
	}
}
