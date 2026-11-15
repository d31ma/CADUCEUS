package storage

type ChangeLogRecord struct {
	Id    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Month string `json:"month"`
}
