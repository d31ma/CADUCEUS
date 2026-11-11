import "mesail.dev/packages/dkim"

// @Delegate
// @NeedsAll()
type DkimKeyDelegate struct {
	settings *SettingsDelegate
}

func (keys *DkimKeyDelegate) Key(domain string, selector string) (dkim.Key, error) {
	return dkim.KeyStore{Source: keys.source(keys.settings.Settings())}.Key(domain, selector)
}

func (DkimKeyDelegate) source(settings MailOutSettings) dkim.SecretSource {
	if settings.SecretFolder != "" {
		return dkim.FolderSecretSource{Root: settings.SecretFolder}
	}
	return dkim.EnvironmentSecretSource{Prefix: settings.SecretPrefix}
}
