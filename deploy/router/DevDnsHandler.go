package main

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type DevDnsHandler struct {
	Zone DevDnsZone
}

func (handler DevDnsHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.reply(request).Send(writer)
}

func (handler DevDnsHandler) reply(request *http.Request) Reply {
	if request.Method != http.MethodGet {
		return Problems{}.Reply(Problems{}.OnlyGet())
	}
	return handler.answer(request.URL.Query())
}

func (handler DevDnsHandler) answer(query url.Values) Reply {
	body, _ := json.Marshal(handler.Zone.Response(query.Get("name"), query.Get("type")))
	header := http.Header{}
	header.Set("Content-Type", "application/dns-json")
	header.Set("Cache-Control", "no-store")
	return Reply{Status: http.StatusOK, Header: header, Body: body}
}
