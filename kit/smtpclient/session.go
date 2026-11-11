package smtpclient

import (
	"cmp"
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"net/textproto"
)

type session struct {
	settings  sessionSettings
	ctx       context.Context
	envelope  Envelope
	message   []byte
	network   net.Conn
	client    *smtp.Client
	encrypted bool
	accepted  Reply
	failures  failures
	stop      func() bool
}

func newSession(settings sessionSettings, envelope Envelope, message []byte) *session {
	return &session{settings: settings, envelope: envelope, message: message, stop: func() bool { return false }}
}

func (session *session) run(ctx context.Context) (Reply, error) {
	session.ctx = ctx
	defer session.close()
	steps := []func() error{session.connect, session.hello, session.startTls, session.authenticate, session.mail, session.recipients, session.data, session.quit}
	return session.finish(session.firstError(steps))
}

func (session *session) firstError(steps []func() error) error {
	var err error
	for index := 0; index < len(steps) && err == nil; index++ {
		err = steps[index]()
	}
	return err
}

func (session *session) finish(err error) (Reply, error) {
	if err != nil {
		failure := session.failures.wrap(cmp.Or(session.ctx.Err(), err), session.settings.Host)
		return failure.Reply, failure
	}
	return session.accepted, nil
}

func (session *session) close() {
	session.stop()
	if session.network != nil {
		session.network.Close()
	}
}

func (session *session) connect() error {
	network, err := session.settings.dialer().DialContext(session.ctx, "tcp", session.settings.Address)
	if err != nil {
		return err
	}
	session.network = network
	session.stop = context.AfterFunc(session.ctx, func() { network.Close() })
	return session.greet()
}

func (session *session) greet() error {
	if err := session.implicitTls(); err != nil {
		return err
	}
	client, err := smtp.NewClient(session.network, session.settings.Host)
	session.client = client
	return err
}

func (session *session) implicitTls() error {
	if !session.settings.ImplicitTls {
		return nil
	}
	secured := tls.Client(session.network, session.settings.tlsConfig())
	session.network = secured
	session.encrypted = true
	return secured.HandshakeContext(session.ctx)
}

func (session *session) hello() error {
	return session.client.Hello(session.settings.heloName())
}

func (session *session) startTls() error {
	offered, _ := session.client.Extension("STARTTLS")
	if session.encrypted || !offered {
		return session.withoutStartTls()
	}
	session.encrypted = true
	return session.client.StartTLS(session.settings.tlsConfig())
}

func (session *session) withoutStartTls() error {
	if session.settings.RequireTls && !session.encrypted {
		return session.failures.tlsMissing()
	}
	return nil
}

func (session *session) authenticate() error {
	if session.settings.Username == "" {
		return nil
	}
	return session.plainAuth()
}

func (session *session) plainAuth() error {
	if !session.encrypted {
		return session.failures.authWithoutTls()
	}
	settings := session.settings
	return session.client.Auth(smtp.PlainAuth("", settings.Username, settings.Password, settings.Host))
}

func (session *session) mail() error {
	return session.client.Mail(session.envelope.From)
}

func (session *session) recipients() error {
	var err error
	recipients := session.envelope.Recipients
	for index := 0; index < len(recipients) && err == nil; index++ {
		err = session.client.Rcpt(recipients[index])
	}
	return err
}

func (session *session) data() error {
	text := session.client.Text
	id, err := text.Cmd("DATA")
	if err != nil {
		return err
	}
	text.StartResponse(id)
	_, _, err = text.ReadResponse(354)
	text.EndResponse(id)
	if err != nil {
		return err
	}
	return session.body(text)
}

func (session *session) body(text *textproto.Conn) error {
	writer := text.DotWriter()
	_, writeError := writer.Write(session.message)
	if err := cmp.Or(writeError, writer.Close()); err != nil {
		return err
	}
	code, message, err := text.ReadResponse(250)
	session.accepted = session.failures.parse(code, message)
	return err
}

func (session *session) quit() error {
	session.client.Quit()
	return nil
}
