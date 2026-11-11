package compose

type fieldNames struct{}

func (fieldNames) From() string       { return "From" }
func (fieldNames) To() string         { return "To" }
func (fieldNames) Cc() string         { return "Cc" }
func (fieldNames) ReplyTo() string    { return "Reply-To" }
func (fieldNames) Subject() string    { return "Subject" }
func (fieldNames) Date() string       { return "Date" }
func (fieldNames) MessageId() string  { return "Message-ID" }
func (fieldNames) InReplyTo() string  { return "In-Reply-To" }
func (fieldNames) References() string { return "References" }
func (fieldNames) Attachment() string { return "Attachment" }
func (fieldNames) Domain() string     { return "domain" }
