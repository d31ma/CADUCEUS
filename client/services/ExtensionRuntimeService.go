// ExtensionRuntimeService runs the signed-in user's enabled extensions, as
// shared/scripts/extension-runtime.js did: the plugins in one opaque sandbox
// iframe (rendered by the shell from HostDocument), each in its own worker,
// their commands as buttons in their page slot (Toolbar), and the enabled
// theme's colours. The extension
// bridge (ExtensionPackageService.js) carries messages between Go and the
// sandboxes and sets the theme's CSS variables; every decision is made here.
// @Service
type ExtensionRuntimeService struct {
	// @Needs
	extensionsClient          *ExtensionsClient
	applicationEventPublisher ApplicationEventPublisher
	Toolbar                   []ExtensionCommand
	HasPlugins                bool
	SandboxTitle              string
	HostDocument              string
	sandboxes                 []ExtensionSandbox
	authorization             string
	signedIn                  bool
	plugins                   []InstalledExtension
	sandboxTokens             map[string]string
	ready                     map[string]bool
}

// @OnStart
func (runtime *ExtensionRuntimeService) Start() {
	runtime.reset()
	runtime.HostDocument = ExtensionSandboxUtils{}.HostDocument(IdentityUtils{}.UUID("extension-host"))
}

// HearSession runs the extensions of the signed-in user, and none after
// sign-out.
// @OnEvent({"replay":true})
func (runtime *ExtensionRuntimeService) HearSession(changed SessionChanged) {
	runtime.authorization = "Bearer " + changed.Token
	runtime.signedIn = changed.SignedIn
	runtime.load()
}

// HearChange runs the extensions again after Settings installed, enabled,
// disabled or removed one.
// @OnEvent
func (runtime *ExtensionRuntimeService) HearChange(changed ExtensionsChanged) {
	runtime.load()
}

// HearAnswer takes what a sandbox said, from the sandbox Go started.
// @OnEvent
func (runtime *ExtensionRuntimeService) HearAnswer(answered ExtensionAnswered) {
	reply := ExtensionSandboxUtils{}.Reply(answered.Message)
	known := ChoiceUtils{}.All(reply.Nonce != "", runtime.sandboxTokens[reply.ExtensionID] == reply.Nonce)
	FlowUtils{}.When(known, runtime.answer(reply))()
}

// Invoke sends a command button's command to its plugin.
func (runtime *ExtensionRuntimeService) Invoke(extensionID string, commandID string, slot string) {
	runtime.applicationEventPublisher.PublishEvent(ExtensionCommandRequested{
		ExtensionID: extensionID, Nonce: runtime.sandboxTokens[extensionID], RequestID: IdentityUtils{}.UUID(extensionID + commandID),
		CommandID: commandID, Slot: slot,
	}, false)
}

func (runtime *ExtensionRuntimeService) reset() {
	runtime.Toolbar = []ExtensionCommand{}
	runtime.sandboxes = []ExtensionSandbox{}
	runtime.plugins = []InstalledExtension{}
	runtime.sandboxTokens = map[string]string{}
	runtime.ready = map[string]bool{}
}

func (runtime *ExtensionRuntimeService) load() {
	runtime.reset()
	extensions := []InstalledExtension{}
	FlowUtils{}.When(runtime.signedIn, func() { extensions, _ = runtime.extensionsClient.Extensions(runtime.authorization) })()
	theme := ExtensionSandboxUtils{}.ThemeVariables(ExtensionSandboxUtils{}.EnabledTheme(extensions))
	plugins := ExtensionSandboxUtils{}.EnabledPlugins(extensions)
	for _, plugin := range plugins {
		runtime.activate(plugin)
	}
	runtime.refresh()
	runtime.HasPlugins = len(runtime.sandboxes) > 0
	runtime.SandboxTitle = ExtensionSandboxUtils{}.Title(runtime.plugins)
	runtime.applicationEventPublisher.PublishEvent(ExtensionPluginsLoaded{Plugins: runtime.sandboxes}, true)
	runtime.applicationEventPublisher.PublishEvent(ExtensionThemeApplied{Variables: theme}, true)
	runtime.applicationEventPublisher.PublishEvent(DocumentAttributesChanged{
		Owner: "extension-theme", Attributes: map[string]string{"data-extension-theme": ChoiceUtils{}.Text(len(theme) > 0, "enabled", "")},
	}, false)
}

// activate reads a plugin's source and gives it a sandbox with a fresh nonce.
func (runtime *ExtensionRuntimeService) activate(projection InstalledExtension) {
	extension, failure := runtime.extensionsClient.Extension(runtime.authorization, projection.ID)
	usable := ChoiceUtils{}.All(failure == nil, extension.Enabled, extension.Source != "")
	FlowUtils{}.When(usable, func() {
		nonce := IdentityUtils{}.UUID(extension.ID + extension.Digest)
		runtime.sandboxTokens[extension.ID] = nonce
		runtime.plugins = append(runtime.plugins, extension)
		runtime.sandboxes = append(runtime.sandboxes, ExtensionSandboxUtils{}.Sandbox(extension, nonce))
	})()
}

func (runtime *ExtensionRuntimeService) refresh() {
	runtime.Toolbar = ExtensionSandboxUtils{}.Toolbar(runtime.plugins, runtime.ready)
}

func (runtime *ExtensionRuntimeService) answer(answered ExtensionReply) func() {
	steps := map[string]func(){
		"ready":   func() { runtime.markReady(answered.ExtensionID) },
		"result":  func() { runtime.notify(answered) },
		"error":   func() { runtime.fail(answered) },
		"stopped": func() { runtime.stop(answered) },
	}
	return FlowUtils{}.Either(steps[answered.Kind] != nil, steps[answered.Kind], func() {})
}

func (runtime *ExtensionRuntimeService) markReady(extensionID string) {
	runtime.ready[extensionID] = true
	runtime.refresh()
}

func (runtime *ExtensionRuntimeService) notify(answered ExtensionReply) {
	message := ExtensionSandboxUtils{}.Notification(ExtensionSandboxUtils{}.Permissions(runtime.plugins, answered.ExtensionID), answered.Result)
	FlowUtils{}.When(message != "", func() {
		runtime.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
	})()
}

func (runtime *ExtensionRuntimeService) fail(answered ExtensionReply) {
	message := ExtensionSandboxUtils{}.Failure(answered.RequestID, answered.Error)
	runtime.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (runtime *ExtensionRuntimeService) stop(answered ExtensionReply) {
	runtime.ready[answered.ExtensionID] = false
	runtime.refresh()
	runtime.fail(answered)
}
