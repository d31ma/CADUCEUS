package smtpclient

import (
	"errors"
	"net/textproto"
	"regexp"
)

type failures struct{}

func (failures) enhancedPattern() *regexp.Regexp {
	return regexp.MustCompile(`(?s)^(?:([245]\.[0-9]{1,3}\.[0-9]{1,3})(?:\s+|$))?(.*)$`)
}

func (failures failures) wrap(err error, host string) DeliveryError {
	var known DeliveryError
	if errors.As(err, &known) {
		return known
	}
	return failures.classify(err, host)
}

func (failures failures) classify(err error, host string) DeliveryError {
	var protocol *textproto.Error
	if errors.As(err, &protocol) {
		reply := failures.parse(protocol.Code, protocol.Msg)
		return DeliveryError{Reply: reply, Host: host, Permanent: reply.Permanent(), Cause: err}
	}
	return failures.temporary(host, err)
}

func (failures failures) parse(code int, text string) Reply {
	parts := failures.enhancedPattern().FindStringSubmatch(text)
	return Reply{Code: code, Enhanced: parts[1], Text: parts[2]}
}

func (failures) temporary(host string, cause error) DeliveryError {
	return DeliveryError{Reply: Reply{Text: cause.Error()}, Host: host, Cause: cause}
}

func (failures) permanent(host string, reply Reply) DeliveryError {
	return DeliveryError{Reply: reply, Host: host, Permanent: true}
}

func (failures) tlsMissing() error {
	return errors.New("STARTTLS is required but the server does not offer it")
}

func (failures) authWithoutTls() error {
	return errors.New("refusing to send credentials over a connection without TLS")
}

func (failures) noHosts() error { return errors.New("no mail hosts to try") }

func (failures) nullMx() Reply {
	return Reply{Code: 556, Enhanced: "5.1.10", Text: "domain does not accept mail (null MX)"}
}

func (failures) badEnvelope() Reply {
	return Reply{Code: 554, Enhanced: "5.5.4", Text: "recipients must be present and share one domain"}
}
