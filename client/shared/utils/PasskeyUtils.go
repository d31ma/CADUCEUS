import "strings"

// PasskeyUtils turns a failed passkey ceremony into the message the login
// form shows.
type PasskeyUtils struct{}

// Failure reads "cancelled" or "error:<message>" from PasskeyService.
func (PasskeyUtils) Failure(failure string, mode string) string {
	return PasskeyUtils{}.message(PasskeyUtils{}.cancellation(failure == "cancelled", mode), strings.TrimPrefix(failure, "error:"))
}

func (PasskeyUtils) cancellation(cancelled bool, mode string) string {
	switch {
	case !cancelled:
		return ""
	case mode == "create":
		return "Passkey registration was cancelled."
	}
	return "Passkey authentication was cancelled."
}

func (PasskeyUtils) message(cancellation string, detail string) string {
	if cancellation != "" {
		return cancellation
	}
	return "Passkey error: " + detail
}
