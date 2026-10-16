// Compact app-owned draft provenance; the editor verifies canonical content before use.
type QuoteProof struct {
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
}
