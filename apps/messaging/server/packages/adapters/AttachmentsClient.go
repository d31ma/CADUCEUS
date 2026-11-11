package adapters

import (
	"net/http"
	"net/url"
	"strings"

	"mesail.dev/packages/core"
)

type AttachmentsClient struct {
	baseUrl string
	tokens  ServiceAuthorizer
	call    JsonCall
}

func NewAttachmentsClient(baseUrl string, tokens ServiceAuthorizer, call JsonCall) AttachmentsClient {
	return AttachmentsClient{baseUrl: strings.TrimSuffix(baseUrl, "/"), tokens: tokens, call: call}
}

func (client AttachmentsClient) Describe(domainId string, attachmentId string) (core.AttachmentInfo, bool, error) {
	authorization, err := client.tokens.Authorization(domainId)
	if err != nil {
		return core.AttachmentInfo{}, false, &core.Unavailable{Message: "no service token for the domain", Cause: err}
	}
	info := core.AttachmentInfo{}
	request := JsonRequest{Method: http.MethodGet, Url: client.metaUrl(attachmentId), Authorization: authorization}
	found, err := client.call.Send(request, &info)
	return info, found, err
}

func (client AttachmentsClient) metaUrl(attachmentId string) string {
	return client.baseUrl + client.attachmentsPath() + url.PathEscape(attachmentId) + client.metaPart()
}

func (AttachmentsClient) attachmentsPath() string {
	return "/api/v1/attachments/"
}

func (AttachmentsClient) metaPart() string {
	return "/meta"
}
