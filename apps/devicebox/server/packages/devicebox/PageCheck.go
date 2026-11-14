package devicebox

import "strconv"

type PageCheck struct{}

func (check PageCheck) After(text string) (int64, error) {
	if text == "" {
		return 0, nil
	}
	return check.cursor(strconv.ParseInt(text, 10, 64))
}

func (check PageCheck) Limit(text string) (int, error) {
	if text == "" {
		return Limits{}.DefaultPage(), nil
	}
	return check.size(strconv.Atoi(text))
}

func (PageCheck) cursor(value int64, err error) (int64, error) {
	if err != nil || value < 0 {
		return 0, InvalidRequest{}.After()
	}
	return value, nil
}

func (PageCheck) size(value int, err error) (int, error) {
	if err != nil || value < 1 || value > (Limits{}).MaxPage() {
		return 0, InvalidRequest{}.Limit()
	}
	return value, nil
}
