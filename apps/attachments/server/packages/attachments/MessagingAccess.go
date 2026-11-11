package attachments

import (
	"net/http"
	"net/url"
)

type MessagingAccess struct {
	BaseUrl string
	Client  *http.Client
}

func (access MessagingAccess) MayRead(attachmentId string, authorization string) bool {
	request, err := http.NewRequest(http.MethodGet, access.BaseUrl+"/api/v1/messaging/attachments/"+url.PathEscape(attachmentId)+"/access", nil)
	if err != nil {
		return false
	}
	request.Header.Set("Authorization", authorization)
	response, err := access.Client.Do(request)
	if err != nil {
		return false
	}
	response.Body.Close()
	return access.success(response.StatusCode)
}

func (MessagingAccess) success(status int) bool {
	return status >= http.StatusOK && status < http.StatusMultipleChoices
}
