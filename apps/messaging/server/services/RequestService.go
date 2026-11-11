import (
	"net/http"
	"strconv"

	"mesail.dev/packages/core"
)

// @Service
type RequestService struct {
}

func (service *RequestService) Authorization(request *http.Request) string {
	return request.Header.Get(service.authorizationName())
}

func (service *RequestService) Page(request *http.Request) (core.PageRequest, error) {
	values := request.URL.Query()
	limit, err := service.limit(values.Get(service.limitName()))
	return core.PageRequest{Cursor: values.Get(service.cursorName()), Limit: limit}, err
}

func (service *RequestService) MessageQuery(conversationId string, request *http.Request) (MessageQuery, error) {
	page, err := service.Page(request)
	threadRootId := request.URL.Query().Get(service.threadRootName())
	return MessageQuery{ConversationId: conversationId, ThreadRootId: threadRootId, Page: page}, err
}

func (service *RequestService) limit(raw string) (int, error) {
	limit, failure := strconv.Atoi(raw)
	switch {
	case raw == "":
		return 0, nil
	case failure != nil, limit < 1, limit > service.largestLimit():
		return 0, &core.InvalidRequest{Field: service.limitName(), Message: "limit must be a whole number from 1 to 100"}
	}
	return limit, nil
}

func (service *RequestService) authorizationName() string {
	return "Authorization"
}

func (service *RequestService) limitName() string {
	return "limit"
}

func (service *RequestService) cursorName() string {
	return "cursor"
}

func (service *RequestService) threadRootName() string {
	return "threadRootId"
}

func (service *RequestService) largestLimit() int {
	return 100
}
