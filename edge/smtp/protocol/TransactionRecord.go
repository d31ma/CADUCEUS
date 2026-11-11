package protocol

import (
	"fmt"
	"strings"
)

type TransactionRecord struct {
	SessionId  string
	Remote     string
	Secure     bool
	Recipients int
	Size       int
	Reply      Reply
	InboundIds []string
	Failure    error
}

func (record TransactionRecord) Line() string {
	return fmt.Sprintf("smtp transaction session=%s remote=%s tls=%t recipients=%d size=%d reply=%d inbound=%s failure=%q",
		record.SessionId, record.Remote, record.Secure, record.Recipients, record.Size, record.Reply.Code,
		strings.Join(record.InboundIds, ","), record.failureText())
}

func (record TransactionRecord) failureText() string {
	if record.Failure == nil {
		return ""
	}
	return record.Failure.Error()
}
