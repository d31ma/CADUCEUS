package notify

import "testing"

func TestPageLimitDefaultsToFifty(t *testing.T) {
	limit, err := PageRequest{}.Limit("")
	expectNoError(t, err)
	expectSame(t, limit, 50)
}

func TestPageLimitAcceptsOne(t *testing.T) {
	limit, err := PageRequest{}.Limit("1")
	expectNoError(t, err)
	expectSame(t, limit, 1)
}

func TestPageLimitAcceptsHundred(t *testing.T) {
	limit, err := PageRequest{}.Limit("100")
	expectNoError(t, err)
	expectSame(t, limit, 100)
}

func TestPageLimitRefusesZero(t *testing.T) {
	_, err := PageRequest{}.Limit("0")
	expectInvalidPage(t, err)
}

func TestPageLimitRefusesHundredAndOne(t *testing.T) {
	_, err := PageRequest{}.Limit("101")
	expectInvalidPage(t, err)
}

func TestPageLimitRefusesText(t *testing.T) {
	_, err := PageRequest{}.Limit("ten")
	expectInvalidPage(t, err)
}

func TestInvalidPageMessage(t *testing.T) {
	expectSame(t, InvalidPage{Reason: "bad"}.Error(), "the page request is not valid: bad")
}
