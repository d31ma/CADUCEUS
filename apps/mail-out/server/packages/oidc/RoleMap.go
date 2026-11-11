package oidc

type RoleMap struct{}

func (roles RoleMap) MesailRoles(claims Claims) []string {
	return roles.translate(claims.RealmRoles, roles.table())
}

func (RoleMap) table() map[string]string {
	return map[string]string{
		RealmRoles{}.Member():   "Member",
		RealmRoles{}.Admin():    "DomainAdmin",
		RealmRoles{}.Operator(): "Operator",
		RealmRoles{}.Service():  "Service",
	}
}

func (RoleMap) translate(realmRoles []string, table map[string]string) []string {
	mesailRoles := []string{}
	for _, realmRole := range realmRoles {
		if mesailRole, known := table[realmRole]; known {
			mesailRoles = append(mesailRoles, mesailRole)
		}
	}
	return mesailRoles
}
