import (
	"cmp"

	"mesail.dev/packages/apierrors"
)

// @ErrorHandler
type MessagingErrorHandler struct {
}

// @HandlesError
// @ResponseStatus(400)
func (handler *MessagingErrorHandler) Invalid(failure *InvalidRequest) ApiError {
	return ApiError{Code: "invalid_request", Message: handler.message(failure, failure.Message), Field: failure.Field}
}

// @HandlesError
// @ResponseStatus(401)
func (handler *MessagingErrorHandler) Unauthenticated(failure *Unauthenticated) ApiError {
	return handler.answer("unauthenticated", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(403)
func (handler *MessagingErrorHandler) Forbidden(failure *Forbidden) ApiError {
	return handler.answer("forbidden", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(404)
func (handler *MessagingErrorHandler) Missing(failure *NotFound) ApiError {
	return handler.answer("not_found", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) Conflict(failure *Conflict) ApiError {
	return handler.answer(cmp.Or(failure.Code, "conflict"), failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) EditConflict(failure *ChangedMessage) EditConflict {
	return EditConflict{Code: "edit_conflict", Message: handler.message(failure, failure.Message), Current: failure.Current}
}

// @HandlesError
// @ResponseStatus(409)
func (handler *MessagingErrorHandler) RecipientsChanged(failure *ChangedRecipients) RecipientsChanged {
	return RecipientsChanged{Code: "recipients_changed", Message: handler.message(failure, failure.Message), Addresses: append([]string{}, failure.Addresses...)}
}

// @HandlesError
// @ResponseStatus(413)
func (handler *MessagingErrorHandler) TooLarge(failure *TooLarge) ApiError {
	return handler.answer("too_large", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(422)
func (handler *MessagingErrorHandler) ChatNotAllowed(failure *ChatNotAllowed) ApiError {
	return ApiError{Code: "chat_not_allowed", Message: handler.message(failure, failure.Message), Field: failure.Address}
}

// @HandlesError
// @ResponseStatus(503)
func (handler *MessagingErrorHandler) Unavailable(failure *Unavailable) ApiError {
	return handler.answer("unavailable", failure, failure.Message)
}

// @HandlesError
// @ResponseStatus(500)
func (handler *MessagingErrorHandler) Unexpected(failure error) ApiError {
	return handler.Unavailable(Unavailable{}.Unexpected())
}

func (handler *MessagingErrorHandler) answer(code string, failure apierrors.Numbered, text string) ApiError {
	return ApiError{Code: code, Message: handler.message(failure, text)}
}

func (handler *MessagingErrorHandler) message(failure apierrors.Numbered, text string) string {
	return apierrors.Message{}.Format(failure.Number(), text)
}
