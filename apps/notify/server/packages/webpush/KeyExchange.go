package webpush

type KeyExchange struct {
	SharedSecret    []byte
	AuthSecret      []byte
	UserAgentPublic []byte
	ServerPublic    []byte
	Salt            []byte
}
