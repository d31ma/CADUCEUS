import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// CalendarReplyMail sends the iCalendar REPLY an RSVP owes an organizer. It
// is the calendar slice's private stand-in for the outbound mail service:
// with SMTP_ADAPTER=console (the default outside production) it logs the
// message and succeeds, as the console adapter did. Direct delivery belongs
// to the send routes, which build and sign MIME messages; until they are
// wired in here, any other adapter answers an error and the RSVP is stored
// without a reply.
type CalendarReplyMail struct{}

// Send delivers one message from an address: {to, subject, text,
// attachments: [{filename, contentBase64, contentType}]}, the shape the
// former SmtpService.sendEmail took.
func (mail CalendarReplyMail) Send(from string, message map[string]any) error {
	switch mail.adapter() {
	case "console":
		to, _ := message["to"].([]string)
		attachments, _ := message["attachments"].([]map[string]any)
		fmt.Fprintf(os.Stderr, "[smtp] from=%s to=%s subject=\"%s\" attachments=%d\n", from, strings.Join(to, ", "), RecordValues{}.String(message["subject"]), len(attachments))
		return nil
	case "direct":
		return errors.New("direct SMTP delivery of calendar replies is not available yet")
	}
	return fmt.Errorf("Unsupported SMTP_ADAPTER=%s. Use direct or console.", mail.adapter())
}

func (CalendarReplyMail) adapter() string {
	settings := Settings{}
	if mode := settings.Value("SMTP_ADAPTER"); mode != "" {
		return mode
	}
	if settings.Value("NODE_ENV") == "production" {
		return "direct"
	}
	return "console"
}
