package reception

type HeaderReader interface {
	Headers(raw []byte) (InboundHeaders, error)
}
