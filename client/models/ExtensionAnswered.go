// ExtensionAnswered is one message a plugin sandbox posted to the page, as
// JSON text; ExtensionRuntimeService reads and checks it.
type ExtensionAnswered struct {
	Message string `json:"message"`
}
