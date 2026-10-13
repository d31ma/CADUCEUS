import "net/http"

// SendUndoRoute answers POST /send/undo.
type SendUndoRoute struct {
	Access *AccessHelper
	Send   *SendService
}

// Undo answers POST /send/undo.
func (route SendUndoRoute) Undo(request *http.Request) *http.Response {
	caller, body, refusal := (DeliveryReply{}).Caller(route.Access, request)
	if refusal != nil {
		return refusal
	}
	undoId, _ := body["undoId"].(string)
	if undoId == "" {
		return Answer{}.BadRequest("undoId is required")
	}
	return DeliveryReply{}.Settled(route.Send.Undo().Undo(caller, undoId))
}
