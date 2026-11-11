type DirectorySettings struct {
	PublicUrl    string                 `json:"publicurl"`
	KeycloakUrl  string                 `json:"keycloakurl"`
	Admin        DirectoryAdminSettings `json:"admin"`
	RealmSecrets DirectoryRealmSecrets  `json:"realmsecrets"`
	MailHost     string                 `json:"mailhost"`
	SpfMechanism string                 `json:"spfmechanism"`
	DkimSelector string                 `json:"dkimselector"`
	DnsResolver  string                 `json:"dnsresolver"`
}
