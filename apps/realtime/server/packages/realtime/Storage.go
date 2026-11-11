package realtime

import "encoding/json"

type Storage struct {
	Documents Documents
	Scroll    Scroller
}

func (storage Storage) Page(query Query, target any) (string, error) {
	window, err := storage.Scroll(query.Collection, query.Where, query.Cursor, query.Size)
	if err != nil {
		return "", err
	}
	return window.NextCursor, json.Unmarshal(window.Content, target)
}
