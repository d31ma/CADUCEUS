import (
	"net/http"
	"net/url"
	"slices"

	"mesail.dev/packages/oidc"
)

// @Service
type ReaderAccessService struct {
}

func (service *ReaderAccessService) Check(caller oidc.Caller, request AccessRequest) error {
	granted := service.grantedHere(caller, request.Record)
	question := service.question(granted, request)
	response, failure := service.send(question)
	status := service.status(response, failure)
	return service.refusal(granted, status)
}

func (service *ReaderAccessService) grantedHere(caller oidc.Caller, record AttachmentRecord) bool {
	return caller.MemberId == record.UploaderId || slices.Contains(caller.Roles, service.serviceRole())
}

func (service *ReaderAccessService) question(granted bool, request AccessRequest) *http.Request {
	target := request.Messaging + "/api/v1/messaging/attachments/" + url.PathEscape(request.AttachmentId) + "/access"
	question, failure := http.NewRequest(http.MethodGet, target, nil)
	if granted || failure != nil {
		return nil
	}
	question.Header.Set("Authorization", request.Authorization)
	return question
}

func (service *ReaderAccessService) send(question *http.Request) (*http.Response, error) {
	if question == nil {
		return nil, &AttachmentNotFound{}
	}
	return http.DefaultClient.Do(question)
}

func (service *ReaderAccessService) status(response *http.Response, failure error) int {
	if failure != nil {
		return http.StatusBadGateway
	}
	response.Body.Close()
	return response.StatusCode
}

func (service *ReaderAccessService) refusal(granted bool, status int) error {
	if !granted && (status < http.StatusOK || status >= http.StatusMultipleChoices) {
		return &AttachmentNotFound{}
	}
	return nil
}

func (service *ReaderAccessService) serviceRole() string {
	return "Service"
}
