package outbox

type Ref struct {
	Collection string `json:"collection"`
	Id         string `json:"id"`
}
