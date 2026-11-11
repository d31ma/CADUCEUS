package sink

type Received struct {
	From       string
	Recipients []string
	Data       []byte
	Tls        bool
	AuthUser   string
}
