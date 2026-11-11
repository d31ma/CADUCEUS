package protocol

import (
	"errors"

	"mesail.dev/edge/smtp/reception"
)

type Replies struct{}

func (replies Replies) Greeting(hostname string) Reply {
	return replies.one(replies.serviceReady(), hostname+" ESMTP Mesail")
}

func (replies Replies) Hello(hostname string) Reply {
	return replies.one(replies.done(), hostname+" Hello")
}

func (replies Replies) ExtendedHello(hostname string, extensions []string) Reply {
	return Reply{Code: replies.done(), Lines: append([]string{hostname + " Hello"}, extensions...)}
}

func (replies Replies) Ok() Reply {
	return replies.one(replies.done(), "2.0.0 OK")
}

func (replies Replies) SenderOk() Reply {
	return replies.one(replies.done(), "2.1.0 Sender OK")
}

func (replies Replies) RecipientOk() Reply {
	return replies.one(replies.done(), "2.1.5 Recipient OK")
}

func (replies Replies) Accepted() Reply {
	return replies.one(replies.done(), "2.0.0 Message accepted for delivery")
}

func (replies Replies) StartData() Reply {
	return replies.one(354, "Start mail input; end with <CRLF>.<CRLF>")
}

func (replies Replies) ReadyForTls() Reply {
	return replies.one(replies.serviceReady(), "2.0.0 Ready to start TLS")
}

func (replies Replies) Closing(hostname string) Reply {
	return replies.one(221, "2.0.0 "+hostname+" closing connection")
}

func (replies Replies) CannotVerify() Reply {
	return replies.one(252, "2.5.0 Cannot verify the user, but will accept the message")
}

func (replies Replies) Help() Reply {
	return replies.one(214, "2.0.0 See RFC 5321")
}

func (replies Replies) NotImplemented() Reply {
	return replies.one(502, "5.5.1 Command not implemented")
}

func (replies Replies) SendHelloFirst() Reply {
	return replies.one(replies.badSequence(), "5.5.1 Send EHLO or HELO first")
}

func (replies Replies) SenderAlreadyGiven() Reply {
	return replies.one(replies.badSequence(), "5.5.1 Sender already given")
}

func (replies Replies) SendMailFirst() Reply {
	return replies.one(replies.badSequence(), "5.5.1 Send MAIL first")
}

func (replies Replies) SendRecipientFirst() Reply {
	return replies.one(replies.badSequence(), "5.5.1 Send RCPT first")
}

func (replies Replies) AlreadySecure() Reply {
	return replies.one(replies.badSequence(), "5.5.1 TLS is already active")
}

func (replies Replies) UnknownCommand() Reply {
	return replies.one(replies.commandError(), "5.5.2 Command not recognized")
}

func (replies Replies) LineTooLong() Reply {
	return replies.one(replies.commandError(), "5.5.2 Line too long")
}

func (replies Replies) SyntaxError() Reply {
	return replies.one(501, "5.5.4 Syntax error in parameters")
}

func (replies Replies) TooLarge() Reply {
	return replies.one(552, "5.3.4 Message exceeds maximum permitted size")
}

func (replies Replies) TooManyRecipients() Reply {
	return replies.one(452, "4.5.3 Too many recipients")
}

func (replies Replies) UnknownUser() Reply {
	return replies.one(replies.mailboxUnavailable(), "5.1.1 No such user here")
}

func (replies Replies) RelayDenied() Reply {
	return replies.one(replies.mailboxUnavailable(), "5.7.1 Relay not permitted")
}

func (replies Replies) PolicyRejected() Reply {
	return replies.one(replies.mailboxUnavailable(), "5.7.1 Message rejected by the DMARC policy of the sender domain")
}

func (replies Replies) LookupFailed() Reply {
	return replies.one(replies.localError(), "4.4.3 Directory lookup failed, try again later")
}

func (replies Replies) TemporaryFailure() Reply {
	return replies.one(replies.localError(), "4.3.0 Temporary local delivery failure, try again later")
}

func (replies Replies) TooManyErrors() Reply {
	return replies.one(replies.serviceClosing(), "4.7.0 Too many errors, closing connection")
}

func (replies Replies) TimedOut(hostname string) Reply {
	return replies.one(replies.serviceClosing(), "4.4.2 "+hostname+" timeout, closing connection")
}

func (replies Replies) Busy() Reply {
	return replies.one(replies.serviceClosing(), "4.3.2 Too many connections, try again later")
}

func (replies Replies) forLookup(err error) Reply {
	switch {
	case errors.Is(err, reception.UnknownAddress{}):
		return replies.UnknownUser()
	case errors.Is(err, reception.DomainNotHosted{}):
		return replies.RelayDenied()
	default:
		return replies.LookupFailed()
	}
}

func (replies Replies) forDelivery(err error) Reply {
	switch {
	case err == nil:
		return replies.Accepted()
	case errors.Is(err, reception.Refused{}):
		return replies.PolicyRejected()
	default:
		return replies.TemporaryFailure()
	}
}

func (Replies) one(code int, line string) Reply {
	return Reply{Code: code, Lines: []string{line}}
}

func (Replies) serviceReady() int       { return 220 }
func (Replies) done() int               { return 250 }
func (Replies) badSequence() int        { return 503 }
func (Replies) commandError() int       { return 500 }
func (Replies) mailboxUnavailable() int { return 550 }
func (Replies) localError() int         { return 451 }
func (Replies) serviceClosing() int     { return 421 }
