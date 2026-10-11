// SmtpService delivers outbound mail with the configured adapter (direct to
// MX in production, console elsewhere); see SmtpTransport.
// @Service
type SmtpService struct{}

// SendEmail delivers a mailbox's composed message from an address.
func (*SmtpService) SendEmail(from string, message map[string]any) (map[string]any, error) {
	return SmtpTransport{}.SendEmail(from, message)
}

// Send delivers a system message from the configured sender.
func (*SmtpService) Send(message map[string]any) (map[string]any, error) {
	return SmtpTransport{}.Send(message)
}
