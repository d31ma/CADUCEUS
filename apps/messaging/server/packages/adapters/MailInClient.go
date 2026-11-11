package adapters

import (
	"net/http"
	"net/url"
	"strings"

	"mesail.dev/packages/core"
)

type MailInClient struct {
	baseUrl string
	tokens  ServiceAuthorizer
	call    JsonCall
}

func NewMailInClient(baseUrl string, tokens ServiceAuthorizer, call JsonCall) MailInClient {
	return MailInClient{baseUrl: strings.TrimSuffix(baseUrl, "/"), tokens: tokens, call: call}
}

func (client MailInClient) Content(domainId string, inboundId string) (core.InboundContent, error) {
	content := core.InboundContent{}
	err := client.fetch(domainId, client.messageUrl(inboundId, client.contentPart()), &content)
	return content, err
}

func (client MailInClient) Original(domainId string, inboundId string) (core.Download, error) {
	download := core.Download{}
	err := client.fetch(domainId, client.messageUrl(inboundId, client.originalPart()), &download)
	return download, err
}

func (client MailInClient) fetch(domainId string, address string, target any) error {
	authorization, err := client.tokens.Authorization(domainId)
	if err != nil {
		return &core.Unavailable{Message: "no service token for the domain", Cause: err}
	}
	found, err := client.call.Send(JsonRequest{Method: http.MethodGet, Url: address, Authorization: authorization}, target)
	if err != nil {
		return err
	}
	return client.missingUnless(found)
}

func (client MailInClient) messageUrl(inboundId string, part string) string {
	return client.baseUrl + client.messagesPath() + url.PathEscape(inboundId) + part
}

func (MailInClient) missingUnless(found bool) error {
	if found {
		return nil
	}
	return &core.NotFound{Message: "no such inbound message"}
}

func (MailInClient) messagesPath() string {
	return "/api/v1/mail-in/messages/"
}

func (MailInClient) contentPart() string {
	return "/content"
}

func (MailInClient) originalPart() string {
	return "/original"
}
