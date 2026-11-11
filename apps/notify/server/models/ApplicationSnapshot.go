import "encoding/json"

type ApplicationSnapshot struct {
	Yon json.RawMessage `json:"yon"`
}
