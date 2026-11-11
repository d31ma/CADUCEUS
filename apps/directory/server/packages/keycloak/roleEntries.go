package keycloak

type roleEntries []roleEntry

func (entries roleEntries) names() map[string]bool {
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name] = true
	}
	return names
}
