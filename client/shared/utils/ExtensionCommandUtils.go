// ExtensionCommandUtils turns a plugin's contributions into command buttons.
type ExtensionCommandUtils struct{}

// Commands are a plugin's command buttons, disabled until its sandbox says
// it is ready.
func (ExtensionCommandUtils) Commands(extension InstalledExtension, ready bool) []ExtensionCommand {
	return ExtensionCommandUtils{}.commands(extension, ready)
}

func (ExtensionCommandUtils) commands(extension InstalledExtension, ready bool) []ExtensionCommand {
	commands := []ExtensionCommand{}
	for _, contribution := range extension.Manifest.Contributions {
		commands = append(commands, ExtensionCommand{
			ExtensionID: extension.ID, CommandID: contribution.ID, Label: contribution.Label, Slot: contribution.Slot,
			Title: extension.DisplayName + ": " + contribution.Label, Disabled: !ready,
		})
	}
	return commands
}
