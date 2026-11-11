package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
)

type ResultTranslator struct {
}

func (translator ResultTranslator) Reply(payload []byte) (Reply, error) {
	return translator.choose(payload, translator.isResult(payload))
}

func (ResultTranslator) isResult(payload []byte) bool {
	var probe map[string]json.RawMessage
	return json.Unmarshal(payload, &probe) == nil && probe["statusCode"] != nil
}

func (translator ResultTranslator) choose(payload []byte, isResult bool) (Reply, error) {
	if !isResult {
		return translator.plain(payload), nil
	}
	return translator.decoded(payload)
}

func (translator ResultTranslator) decoded(payload []byte) (Reply, error) {
	var result HttpResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return Reply{}, err
	}
	return translator.translated(result)
}

func (ResultTranslator) plain(payload []byte) Reply {
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	return Reply{Status: http.StatusOK, Header: header, Body: payload}
}

func (translator ResultTranslator) translated(result HttpResult) (Reply, error) {
	body, err := translator.body(result)
	if err != nil {
		return Reply{}, err
	}
	header := http.Header{}
	translator.addSingle(header, result.Headers)
	translator.addMultiple(header, result.MultiValueHeaders)
	translator.addCookies(header, result.Cookies)
	return Reply{Status: result.StatusCode, Header: header, Body: body}, nil
}

func (ResultTranslator) body(result HttpResult) ([]byte, error) {
	if result.IsBase64Encoded {
		return base64.StdEncoding.DecodeString(result.Body)
	}
	return []byte(result.Body), nil
}

func (ResultTranslator) addSingle(header http.Header, values map[string]string) {
	for name, value := range values {
		header.Set(name, value)
	}
}

func (ResultTranslator) addMultiple(header http.Header, values map[string][]string) {
	for name, list := range values {
		header[http.CanonicalHeaderKey(name)] = list
	}
}

func (ResultTranslator) addCookies(header http.Header, cookies []string) {
	for _, cookie := range cookies {
		header.Add("Set-Cookie", cookie)
	}
}
