import (
	"errors"
	"log"
	"strconv"
	"strings"
)

// SmtpTransport delivers outbound mail with the adapter SMTP_ADAPTER names:
// "direct" (straight to each recipient's MX, the production default) or
// "console" (log only, the default outside production).
type SmtpTransport struct{}

// SendEmail delivers a composed message from an address and answers
// {messageId, results?}; a partial or failed delivery is a *SmtpFailure.
func (transport SmtpTransport) SendEmail(from string, message map[string]any) (map[string]any, error) {
	switch transport.mode() {
	case "direct":
		return SmtpDirect{}.SendEmail(from, message)
	case "console":
		log.Printf("[smtp] from=%s to=%s subject=%q attachments=%d", from, transport.recipients(message), RecordValues{}.Text(message["subject"]), transport.attachments(message))
		return map[string]any{"messageId": transport.messageId(message)}, nil
	}
	return nil, errors.New("Unsupported SMTP_ADAPTER=" + transport.mode() + ". Use direct or console.")
}

// Send delivers a system message (a forward) from the configured sender.
func (transport SmtpTransport) Send(message map[string]any) (map[string]any, error) {
	switch transport.mode() {
	case "direct":
		return SmtpDirect{}.Send(message, "")
	case "console":
		log.Printf("[smtp] to=%s subject=%q attachments=%d", transport.recipients(message), RecordValues{}.Text(message["subject"]), transport.attachments(message))
		return map[string]any{"messageId": transport.messageId(message)}, nil
	}
	return nil, errors.New("Unsupported SMTP_ADAPTER=" + transport.mode() + ". Use direct or console.")
}

func (SmtpTransport) mode() string {
	settings := Settings{}
	if mode := settings.Value("SMTP_ADAPTER"); mode != "" {
		return mode
	}
	if settings.Value("NODE_ENV") == "production" {
		return "direct"
	}
	return "console"
}

func (SmtpTransport) recipients(message map[string]any) string {
	return strings.Join(DeliveryFields{}.Strings(message["to"]), ", ")
}

func (SmtpTransport) attachments(message map[string]any) int {
	list, _ := RecordValues{}.List(message["attachments"])
	return len(list)
}

func (SmtpTransport) messageId(message map[string]any) string {
	if id := (RecordValues{}).Text(message["messageId"]); id != "" {
		return id
	}
	return "console-" + strconv.FormatInt(EpochTime{}.Now(), 10)
}
