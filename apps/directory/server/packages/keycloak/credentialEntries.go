package keycloak

type credentialEntries []credentialEntry

func (entries credentialEntries) idsByType() map[string][]string {
	ids := map[string][]string{}
	for _, entry := range entries {
		ids[entry.Type] = append(ids[entry.Type], entry.Id)
	}
	return ids
}
