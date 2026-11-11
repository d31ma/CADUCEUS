package mailout

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Sources struct {
	Client ServiceClient
	Bases  map[string]string
}

func (sources Sources) Knows(producer string) bool {
	_, known := sources.Bases[producer]
	return known
}

func (sources Sources) Source(item Item) (MessageSource, error) {
	base, err := sources.base(item.Producer)
	if err != nil {
		return MessageSource{}, err
	}
	source := MessageSource{}
	status, err := sources.Client.Get(sources.address(base, item), item.DomainId, &source)
	if err != nil {
		return source, err
	}
	return sources.answer(status, source)
}

func (sources Sources) base(producer string) (string, error) {
	base, known := sources.Bases[producer]
	if !known {
		return "", fmt.Errorf("no message source is known for the producer %q", producer)
	}
	return base, nil
}

func (Sources) address(base string, item Item) string {
	return strings.TrimRight(base, "/") + "/api/v1/" + item.Producer + "/messages/" + url.PathEscape(item.MessageId) + "/source"
}

func (Sources) answer(status int, source MessageSource) (MessageSource, error) {
	if status != http.StatusOK {
		return MessageSource{}, fmt.Errorf("the producer answered %d for the message source", status)
	}
	return source, nil
}
