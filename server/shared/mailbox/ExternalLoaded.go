// ExternalLoaded is one owner-authorized provider message: its identity, its
// account and folders, its enriched index row and, when loaded with its
// source, the RFC 822 source in base64.
type ExternalLoaded struct {
	AccountId string
	StableId  string
	Account   map[string]any
	Folders   []any
	Indexed   map[string]any
	RawBase64 any
}
