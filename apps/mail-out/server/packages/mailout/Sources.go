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
	base, known := sources.Bases[item.Producer]
	if !known {
		return MessageSource{}, fmt.Errorf("no message source is known for the producer %q", item.Producer)
	}
	address := strings.TrimRight(base, "/") + "/api/v1/" + item.Producer + "/messages/" + url.PathEscape(item.MessageId) + "/source"
	source := MessageSource{}
	status, err := sources.Client.Get(address, item.DomainId, &source)
	if err == nil && status != http.StatusOK {
		return MessageSource{}, fmt.Errorf("the producer answered %d for the message source", status)
	}
	return source, err
}
