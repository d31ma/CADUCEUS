package keycloak

import "strings"

type userEntries []userEntry

func (entries userEntries) idsByUsername() map[string][]string {
	ids := map[string][]string{}
	for _, entry := range entries {
		key := strings.ToLower(entry.Username)
		ids[key] = append(ids[key], entry.Id)
	}
	return ids
}
