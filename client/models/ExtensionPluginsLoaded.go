// ExtensionPluginsLoaded hands the extension bridge the enabled plugins to
// start in the sandbox host, replacing the ones it ran before.
type ExtensionPluginsLoaded struct {
	Plugins []ExtensionSandbox `json:"plugins"`
}
