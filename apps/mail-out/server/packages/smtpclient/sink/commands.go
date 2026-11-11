package sink

type commands struct{}

func (commands) Gone() string     { return "" }
func (commands) Ehlo() string     { return "EHLO" }
func (commands) Helo() string     { return "HELO" }
func (commands) StartTls() string { return "STARTTLS" }
func (commands) Auth() string     { return "AUTH" }
func (commands) Mail() string     { return "MAIL" }
func (commands) Rcpt() string     { return "RCPT" }
func (commands) Data() string     { return "DATA" }
func (commands) Rset() string     { return "RSET" }
func (commands) Noop() string     { return "NOOP" }
func (commands) Quit() string     { return "QUIT" }
