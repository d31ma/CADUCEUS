import (
	"net/url"
	"strconv"
)

type PageWindow struct{}

func (window PageWindow) Size(limit int) (int, error) {
	return window.checkedSize(limit)
}

func (window PageWindow) Offset(cursor string) (int, error) {
	value, failure := strconv.Atoi(cursor)
	return window.checkedOffset(cursor, value, failure)
}

func (window PageWindow) Next(end int, total int) string {
	if end >= total {
		return ""
	}
	return strconv.Itoa(end)
}

func (window PageWindow) checkedSize(limit int) (int, error) {
	switch {
	case limit == 0:
		return window.defaultLimit(), nil
	case limit < 1, limit > window.maximumLimit():
		return 0, &InvalidRequest{Field: "limit", Message: "limit must be between 1 and 100"}
	}
	return limit, nil
}

func (window PageWindow) checkedOffset(cursor string, value int, failure error) (int, error) {
	switch {
	case cursor == "":
		return 0, nil
	case failure != nil, value < 0:
		return 0, &InvalidRequest{Field: "cursor", Message: "the cursor is not one this service gave out"}
	}
	return value, nil
}

func (window PageWindow) defaultLimit() int {
	return 20
}

func (window PageWindow) maximumLimit() int {
	return 100
}

func (window PageWindow) Query(values url.Values) (MemberQuery, error) {
	limit, failure := strconv.Atoi(values.Get(window.limitName()))
	query := MemberQuery{Text: values.Get("query"), Cursor: values.Get("cursor"), Limit: limit}
	return window.checkedQuery(query, values.Get(window.limitName()), failure)
}

func (window PageWindow) checkedQuery(query MemberQuery, raw string, failure error) (MemberQuery, error) {
	if raw != "" && failure != nil {
		return MemberQuery{}, &InvalidRequest{Field: window.limitName(), Message: "limit must be a whole number"}
	}
	return query, nil
}

func (window PageWindow) limitName() string {
	return "limit"
}
