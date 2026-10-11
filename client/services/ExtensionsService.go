import (
	"cmp"
	"encoding/json"
	"slices"
)

// ExtensionsService inspects a local extension package, installs only the
// exact digest the owner trusted, and enables, disables or removes the
// installed packages. ExtensionPackageService.js reads the chosen file.
// @Service
type ExtensionsService struct {
	// @Needs
	extensionsClient          *ExtensionsClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	bundle                    any
	Extensions                []InstalledExtension
	Inspection                *InstalledExtension
	Busy                      bool
	Error                     string
}

// @OnEvent({"replay":true})
func (service *ExtensionsService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	refused := false
	FlowUtils{}.When(changed.SignedIn, func() { refused = service.reload() })()
	service.announce(refused)
}

// @OnEvent
func (service *ExtensionsService) Inspect(read ExtensionPackageRead) {
	service.Error = read.Problem
	service.Inspection = nil
	service.bundle = nil
	FlowUtils{}.When(ChoiceUtils{}.All(read.Problem == "", read.Text != ""), func() { service.inspect(read.Text) })()
	service.announce(false)
}

// Trust installs the inspected package, disabled, by its exact digest.
func (service *ExtensionsService) Trust() error {
	ready := ChoiceUtils{}.All(service.bundle != nil, service.Inspection != nil)
	FlowUtils{}.When(ready, service.install)()
	return nil
}

func (service *ExtensionsService) Toggle(id string) error {
	extension, found := service.extension(id)
	FlowUtils{}.Either(found, func() { service.toggle(extension) }, service.changed)()
	return nil
}

// @OnEvent
func (service *ExtensionsService) Remove(given ConfirmationGiven) {
	extension, found := service.extension(given.ID)
	FlowUtils{}.When(given.Action == "extension-remove", func() {
		FlowUtils{}.Either(found, func() { service.remove(extension) }, service.changed)()
	})()
	service.announce(false)
}

func (service *ExtensionsService) PermissionLabel(permission string) string {
	labels := map[string]string{
		"compose:read":  "Read the current draft",
		"compose:write": "Change the current draft",
		"mail:metadata": "Read open-message metadata",
		"ui:notify":     "Show a local notification",
	}
	return cmp.Or(labels[permission], permission)
}

func (service *ExtensionsService) inspect(text string) {
	var bundle any
	parsed := json.Unmarshal([]byte(text), &bundle) == nil
	FlowUtils{}.Either(parsed, func() { service.send(bundle) }, func() { service.Error = "Extension package is not valid JSON." })()
}

func (service *ExtensionsService) send(bundle any) {
	service.Busy = true
	inspection, failure := service.extensionsClient.Inspect(service.authorization, map[string]any{service.bundleKey(): bundle})
	service.Busy = false
	FlowUtils{}.Either(failure == nil, func() { service.showInspection(bundle, inspection) }, func() {
		service.Error = HTTPUtils{}.Message(failure, "Extension inspection failed.", service.network())
	})()
}

func (service *ExtensionsService) showInspection(bundle any, inspection InstalledExtension) {
	service.bundle = bundle
	service.Inspection = &inspection
}

func (service *ExtensionsService) install() {
	service.Busy = true
	service.Error = ""
	installed, failure := service.extensionsClient.Install(service.authorization, map[string]any{service.bundleKey(): service.bundle, "trustedDigest": service.Inspection.Digest})
	service.Busy = false
	FlowUtils{}.Either(failure == nil, func() { service.showInstalled(installed) }, func() {
		service.Error = HTTPUtils{}.Message(failure, "Extension installation failed.", service.network())
	})()
}

func (service *ExtensionsService) showInstalled(installed InstalledExtension) {
	service.Inspection = nil
	service.bundle = nil
	service.reload()
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "“" + installed.DisplayName + "” installed disabled."}, false)
}

func (service *ExtensionsService) toggle(extension InstalledExtension) {
	service.Busy = true
	service.Error = ""
	_, failure := service.extensionsClient.Update(service.authorization, extension.ID, map[string]any{"enabled": !extension.Enabled})
	service.Busy = false
	service.showChanged(failure, "Extension update failed.")
}

func (service *ExtensionsService) remove(extension InstalledExtension) {
	service.Busy = true
	service.Error = ""
	_, failure := service.extensionsClient.Remove(service.authorization, extension.ID)
	service.Busy = false
	service.showChanged(failure, "Extension removal failed.")
}

func (service *ExtensionsService) showChanged(failure error, message string) {
	if failure != nil {
		service.Error = HTTPUtils{}.Message(failure, message, service.network())
		return
	}
	service.reload()
}

func (service *ExtensionsService) changed() {
	service.Error = "Extension state changed; reload settings and try again."
}

// reload reads the installed extensions and says whether the server refused the read.
func (service *ExtensionsService) reload() bool {
	extensions, failure := service.extensionsClient.Extensions(service.authorization)
	service.Extensions = append([]InstalledExtension{}, extensions...)
	return HTTPUtils{}.Refused(failure)
}

func (service *ExtensionsService) extension(id string) (InstalledExtension, bool) {
	index := slices.IndexFunc(service.Extensions, func(extension InstalledExtension) bool { return extension.ID == id })
	return append(slices.Clone(service.Extensions), InstalledExtension{})[cmp.Or(index+1, len(service.Extensions)+1)-1], index >= 0
}

func (service *ExtensionsService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "extensions", Refused: refused}, false)
}

func (service *ExtensionsService) network() string { return "Network error." }

func (service *ExtensionsService) bundleKey() string { return "bundle" }
