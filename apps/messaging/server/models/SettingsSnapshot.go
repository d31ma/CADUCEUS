import "encoding/json"

type SettingsSnapshot struct {
	Yon json.RawMessage `json:"yon"`
}
