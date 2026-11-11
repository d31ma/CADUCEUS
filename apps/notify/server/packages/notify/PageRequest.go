package notify

import "strconv"

type PageRequest struct{}

func (request PageRequest) Limit(raw string) (int, error) {
	limit, err := request.parse(request.withDefault(raw))
	if err != nil {
		return 0, err
	}
	return limit, request.checkRange(limit)
}

func (PageRequest) defaultLimit() int {
	return 50
}

func (PageRequest) largestLimit() int {
	return 100
}

func (PageRequest) outOfRange() InvalidPage {
	return InvalidPage{Reason: "limit must be a whole number from 1 to 100"}
}

func (request PageRequest) withDefault(raw string) string {
	if raw == "" {
		return strconv.Itoa(request.defaultLimit())
	}
	return raw
}

func (request PageRequest) parse(text string) (int, error) {
	limit, err := strconv.Atoi(text)
	if err != nil {
		return 0, request.outOfRange()
	}
	return limit, nil
}

func (request PageRequest) checkRange(limit int) error {
	if limit < 1 || limit > request.largestLimit() {
		return request.outOfRange()
	}
	return nil
}
