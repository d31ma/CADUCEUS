import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/inbound"
)

// @ErrorHandler
type MessagingErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) Invalid(failure *core.InvalidRequest) ApiError {
	return ApiError{Code: handler.invalidRequestCode(), Message: failure.Message, Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) InvalidEnvelope(failure events.InvalidEnvelope) ApiError {
	return ApiError{Code: handler.invalidRequestCode(), Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) NotSubscribed(failure events.NotSubscribed) ApiError {
	return ApiError{Code: handler.invalidRequestCode(), Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) UnknownEventType(failure events.UnknownEventType) ApiError {
	return ApiError{Code: handler.invalidRequestCode(), Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) UnhandledEvent(failure inbound.UnhandledEvent) ApiError {
	return ApiError{Code: handler.invalidRequestCode(), Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *MessagingErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return ApiError{Code: "unauthenticated", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *MessagingErrorHandler) Forbidden(failure *core.Forbidden) ApiError {
	return ApiError{Code: handler.forbiddenCode(), Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(403)
func (handler *MessagingErrorHandler) NotService(failure events.NotService) ApiError {
	return ApiError{Code: handler.forbiddenCode(), Message: failure.Error()}
}

// @HandlesError
// @ResponseStatus(404)
func (handler *MessagingErrorHandler) Missing(failure *core.NotFound) ApiError {
	return ApiError{Code: "not_found", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) Conflict(failure *core.Conflict) ApiError {
	return ApiError{Code: "conflict", Message: failure.Message}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) RecipientsChanged(failure *core.RecipientsChanged) RecipientsChanged {
	return RecipientsChanged{Code: "recipients_changed", Message: failure.Error(), Addresses: append([]string{}, failure.Addresses...)}
}

// @HandlesError
// @ResponseStatus(422)
func (handler *MessagingErrorHandler) ChatNotAllowed(failure *core.ChatNotAllowed) ApiError {
	return ApiError{Code: "chat_not_allowed", Message: failure.Error(), Field: failure.Address}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *MessagingErrorHandler) Unavailable(failure *core.Unavailable) ApiError {
	return ApiError{Code: "unavailable", Message: failure.Message}
}

func (handler *MessagingErrorHandler) invalidRequestCode() string {
	return "invalid_request"
}

func (handler *MessagingErrorHandler) forbiddenCode() string {
	return "forbidden"
}
