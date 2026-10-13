import (
	"errors"
	"math"
	"strings"
)

// InboxFilters are the query filters of GET /inbox. Empty texts and nil
// pointers are filters the query did not give.
type InboxFilters struct {
	Query         string
	Folder        string
	Domain        string
	AccountId     string
	Read          *bool
	Starred       *bool
	HasAttachment *bool
	Limit         *int
	Offset        *int
}

// Parsed answers the filters of a query, or the reason one is invalid.
func (filters InboxFilters) Parsed(query map[string]string) (InboxFilters, error) {
	parsed := InboxFilters{}
	search, given := query["q"]
	if !given {
		search = query["search"]
	}
	parsed.Query = filters.text(search)
	parsed.Folder = filters.text(query["folder"])
	parsed.Domain = strings.ToLower(filters.text(query["domain"]))
	parsed.AccountId = filters.text(query["account"])
	if parsed.AccountId != "" && !(ExternalMessage{}).IsAccountId(parsed.AccountId) {
		return parsed, errors.New("Invalid connected account identifier")
	}
	var err error
	if parsed.Read, err = filters.flag(query, "read"); err != nil {
		return parsed, err
	}
	if parsed.Starred, err = filters.flag(query, "starred"); err != nil {
		return parsed, err
	}
	attachments := "hasAttachment"
	if _, given := query[attachments]; !given {
		attachments = "attachments"
	}
	if parsed.HasAttachment, err = filters.flag(query, attachments); err != nil {
		return parsed, err
	}
	if parsed.Limit, err = filters.count(query, "limit"); err != nil {
		return parsed, err
	}
	parsed.Offset, err = filters.count(query, "offset")
	return parsed, err
}

func (InboxFilters) text(value string) string {
	return TextValue{}.Trim(value)
}

func (InboxFilters) flag(query map[string]string, name string) (*bool, error) {
	value := query[name]
	switch value {
	case "":
		return nil, nil
	case "true", "1":
		enabled := true
		return &enabled, nil
	case "false", "0":
		enabled := false
		return &enabled, nil
	}
	return nil, errors.New("Invalid boolean query parameter")
}

func (InboxFilters) count(query map[string]string, name string) (*int, error) {
	value := query[name]
	if value == "" {
		return nil, nil
	}
	parsed := RecordValues{}.Numeric(value)
	if math.IsNaN(parsed) || parsed != math.Trunc(parsed) || parsed < 0 || math.IsInf(parsed, 0) {
		return nil, errors.New("Invalid " + name + " query parameter")
	}
	count := int(parsed)
	return &count, nil
}
