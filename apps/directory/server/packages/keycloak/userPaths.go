package keycloak

import "net/url"

type userPaths struct{}

func (userPaths) Users() string {
	return "/users"
}

func (paths userPaths) Search(username string) string {
	query := url.Values{}
	query.Set("exact", "true")
	query.Set("username", username)
	return paths.Users() + "?" + query.Encode()
}

func (paths userPaths) User(userId string) string {
	return paths.Users() + "/" + url.PathEscape(userId)
}

func (paths userPaths) ResetPassword(userId string) string {
	return paths.User(userId) + "/reset-password"
}

func (paths userPaths) Logout(userId string) string {
	return paths.User(userId) + "/logout"
}

func (paths userPaths) Credentials(userId string) string {
	return paths.User(userId) + "/credentials"
}

func (paths userPaths) Credential(userId string, credentialId string) string {
	return paths.Credentials(userId) + "/" + url.PathEscape(credentialId)
}

func (paths userPaths) Passkeys(userId string) string {
	return paths.mesailEndpoints() + paths.User(userId) + "/passkeys"
}

func (userPaths) mesailEndpoints() string {
	return "/mesail"
}

func (paths userPaths) RealmRoles(userId string) string {
	return paths.User(userId) + "/role-mappings/realm/composite"
}
