import "encoding/json"

type PushOptions struct {
	Enabled json.RawMessage `json:"enabled"`
	Hosts   []string        `json:"hosts"`
}
