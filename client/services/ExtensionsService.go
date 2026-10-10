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
func (service *ExtensionsService) Open(session AccountSession) error {
	service.authorization = "Bearer " + session.Token
	service.reload()
	service.announce()
	return nil
}

// @OnEvent
func (service *ExtensionsService) Inspect(read ExtensionPackageRead) error {
	service.Error = read.Problem
	service.Inspection = nil
	service.bundle = nil
	FlowUtils{}.When(FlowUtils{}.Both(read.Problem == "", read.Text != ""), func() { service.inspect(read.Text) })
	service.announce()
	return nil
}

// Trust installs the inspected package, disabled, by its exact digest.
func (service *ExtensionsService) Trust() error {
	ready := FlowUtils{}.Both(service.bundle != nil, service.Inspection != nil)
	FlowUtils{}.When(ready, service.install)
	return nil
}

func (service *ExtensionsService) Toggle(id string) error {
	extension, found := service.extension(id)
	FlowUtils{}.Choose(found, func() { service.toggle(extension) }, service.changed)
	return nil
}

// @OnEvent
func (service *ExtensionsService) Remove(accepted ConfirmationAccepted) error {
	extension, found := service.extension(accepted.Subject)
	FlowUtils{}.When(accepted.Topic == "extension-remove", func() {
		FlowUtils{}.Choose(found, func() { service.remove(extension) }, service.changed)
	})
	service.announce()
	return nil
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
	FlowUtils{}.Choose(parsed, func() { service.send(bundle) }, func() { service.Error = "Extension package is not valid JSON." })
}

func (service *ExtensionsService) send(bundle any) {
	service.Busy = true
	inspection, failure := service.extensionsClient.Inspect(service.authorization, map[string]any{service.bundleKey(): bundle})
	service.Busy = false
	service.showInspection(bundle, inspection, failure)
}

func (service *ExtensionsService) showInspection(bundle any, inspection InstalledExtension, failure error) {
	if failure != nil {
		service.Error = "Extension inspection failed."
		return
	}
	service.bundle = bundle
	service.Inspection = &inspection
}

func (service *ExtensionsService) install() {
	service.Busy = true
	service.Error = ""
	installed, failure := service.extensionsClient.Install(service.authorization, map[string]any{service.bundleKey(): service.bundle, "trustedDigest": service.Inspection.Digest})
	service.Busy = false
	service.showInstalled(installed, failure)
}

func (service *ExtensionsService) showInstalled(installed InstalledExtension, failure error) {
	if failure != nil {
		service.Error = "Extension installation failed."
		return
	}
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
		service.Error = message
		return
	}
	service.reload()
	service.applicationEventPublisher.PublishEvent(ExtensionsReloadRequested{Count: len(service.Extensions)}, false)
}

func (service *ExtensionsService) changed() {
	service.Error = "Extension state changed; reload settings and try again."
}

func (service *ExtensionsService) reload() {
	extensions, _ := service.extensionsClient.Extensions(service.authorization)
	service.Extensions = append([]InstalledExtension{}, extensions...)
}

func (service *ExtensionsService) extension(id string) (InstalledExtension, bool) {
	index := slices.IndexFunc(service.Extensions, func(extension InstalledExtension) bool { return extension.ID == id })
	return append(slices.Clone(service.Extensions), InstalledExtension{})[cmp.Or(index+1, len(service.Extensions)+1)-1], index >= 0
}

func (service *ExtensionsService) announce() {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "extensions"}, false)
}

func (service *ExtensionsService) bundleKey() string { return "bundle" }
