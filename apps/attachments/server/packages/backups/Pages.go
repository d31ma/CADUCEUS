package backups

import "encoding/json"

type Pages struct {
	Scroll Scroller
}

func (pages Pages) First(collection string, where map[string]any, target any) (bool, error) {
	rows, err := pages.search(collection, where)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, pages.decodeFirst(rows, target)
}

func (pages Pages) search(collection string, where map[string]any) ([]json.RawMessage, error) {
	rows, cursor, more := []json.RawMessage{}, "", true
	for page := 0; more && page < pages.mostPages(); page++ {
		var err error
		rows, cursor, err = pages.read(collection, where, cursor)
		if err != nil {
			return nil, err
		}
		more = pages.keepReading(rows, cursor)
	}
	return rows, nil
}

func (pages Pages) read(collection string, where map[string]any, cursor string) ([]json.RawMessage, string, error) {
	window, err := pages.Scroll(collection, where, cursor, pages.pageSize())
	if err != nil {
		return nil, "", err
	}
	rows, err := pages.rows(window.Content)
	return rows, window.NextCursor, err
}

func (Pages) rows(content json.RawMessage) ([]json.RawMessage, error) {
	rows := []json.RawMessage{}
	if len(content) == 0 {
		return rows, nil
	}
	return rows, json.Unmarshal(content, &rows)
}

func (Pages) keepReading(rows []json.RawMessage, cursor string) bool {
	return len(rows) == 0 && cursor != ""
}

func (Pages) decodeFirst(rows []json.RawMessage, target any) error {
	if len(rows) == 0 {
		return nil
	}
	return json.Unmarshal(rows[0], target)
}

func (Pages) pageSize() int {
	return 100
}

func (Pages) mostPages() int {
	return 20
}
