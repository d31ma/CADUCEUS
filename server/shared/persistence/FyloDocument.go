import "encoding/json"

type FyloDocument struct {
	Id   string
	Data json.RawMessage
}
