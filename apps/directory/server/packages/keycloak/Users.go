package keycloak

import (
	"errors"
	"net/http"
	"path"
	"strings"
)

type Users struct {
	admin adminClient
}

func NewUsers(settings UserSettings, client *http.Client) Users {
	return Users{admin: adminClient{settings: settings, client: client}}
}

func (users Users) Create(user NewUser) (string, error) {
	answer, err := users.admin.write(http.MethodPost, userPaths{}.Users(), users.newUserBody(user))
	if err != nil {
		return "", err
	}
	err = users.createOutcome(answer, user.Username)
	if err != nil {
		return "", err
	}
	return users.createdId(answer, user.Username)
}

func (users Users) IdOf(username string) (string, error) {
	answer, err := users.admin.read(http.MethodGet, userPaths{}.Search(username))
	if err != nil {
		return "", err
	}
	err = users.outcome(answer, http.StatusOK, users.searchAction())
	if err != nil {
		return "", err
	}
	var entries userEntries
	err = users.admin.decode(answer.Content, &entries)
	if err != nil {
		return "", err
	}
	return users.onlyId(entries.idsByUsername()[strings.ToLower(username)])
}

func (users Users) Tagged(username string) (TaggedUser, error) {
	id, err := users.IdOf(username)
	if err != nil {
		return TaggedUser{}, err
	}
	user, err := users.readUser(id)
	if err != nil {
		return TaggedUser{}, err
	}
	tag, err := user.attribute(addRequest{}.Key())
	return TaggedUser{Id: id, AddRequest: tag}, err
}

func (users Users) SetEnabled(userId string, enabled bool) error {
	user, err := users.readUser(userId)
	if err != nil {
		return err
	}
	user.setEnabled(enabled)
	return users.writeUser(userId, user)
}

func (users Users) ResetPassword(userId string, password string) error {
	answer, err := users.admin.write(http.MethodPut, userPaths{}.ResetPassword(userId), users.password(password))
	if err != nil {
		return err
	}
	err = users.passwordOutcome(answer, http.StatusNoContent, users.resetAction())
	if err != nil {
		return err
	}
	err = users.requirePasswordChange(userId)
	if err != nil {
		return err
	}
	return users.logout(userId)
}

func (users Users) RemoveTotp(userId string) error {
	answer, err := users.admin.read(http.MethodGet, userPaths{}.Credentials(userId))
	if err != nil {
		return err
	}
	err = users.outcome(answer, http.StatusOK, users.credentialsAction())
	if err != nil {
		return err
	}
	var credentials credentialEntries
	err = users.admin.decode(answer.Content, &credentials)
	if err != nil {
		return err
	}
	return users.deleteCredentials(userId, credentials.idsByType()[users.totpType()])
}

func (users Users) RemovePasskeys(userId string) error {
	answer, err := users.admin.read(http.MethodDelete, userPaths{}.Passkeys(userId))
	if err != nil {
		return err
	}
	err = users.outcome(answer, http.StatusNoContent, users.removePasskeysAction())
	if err != nil {
		return err
	}
	return users.logout(userId)
}

func (users Users) Delete(userId string) error {
	answer, err := users.admin.read(http.MethodDelete, userPaths{}.User(userId))
	if err != nil {
		return err
	}
	return users.deleteOutcome(answer)
}

func (users Users) IsOperator(userId string) (bool, error) {
	answer, err := users.admin.read(http.MethodGet, userPaths{}.RealmRoles(userId))
	if err != nil {
		return false, err
	}
	err = users.outcome(answer, http.StatusOK, users.rolesAction())
	if err != nil {
		return false, err
	}
	var roles roleEntries
	err = users.admin.decode(answer.Content, &roles)
	if err != nil {
		return false, err
	}
	return roles.names()[users.operatorRole()], nil
}

func (users Users) newUserBody(user NewUser) newUserBody {
	return newUserBody{
		Username:    user.Username,
		Enabled:     true,
		Attributes:  users.newAttributes(user.AddRequest),
		Credentials: []passwordCredential{users.password(user.Password)},
	}
}

func (Users) newAttributes(request string) map[string][]string {
	attributes := map[string][]string{mustChangePassword{}.Key(): mustChangePassword{}.Values()}
	if request != "" {
		attributes[addRequest{}.Key()] = addRequest{}.Values(request)
	}
	return attributes
}

func (users Users) password(value string) passwordCredential {
	return passwordCredential{Type: users.passwordType(), Value: value, Temporary: false}
}

func (users Users) createdId(answer adminAnswer, username string) (string, error) {
	if answer.Location == "" {
		return users.IdOf(username)
	}
	return path.Base(answer.Location), nil
}

func (Users) onlyId(ids []string) (string, error) {
	if len(ids) != 1 {
		return "", UserMissing{}
	}
	return ids[0], nil
}

func (users Users) readUser(userId string) (userRecord, error) {
	answer, err := users.admin.read(http.MethodGet, userPaths{}.User(userId))
	if err != nil {
		return nil, err
	}
	err = users.outcome(answer, http.StatusOK, users.readAction())
	if err != nil {
		return nil, err
	}
	var user userRecord
	err = users.admin.decode(answer.Content, &user)
	return user, err
}

func (users Users) writeUser(userId string, user userRecord) error {
	answer, err := users.admin.write(http.MethodPut, userPaths{}.User(userId), user)
	if err != nil {
		return err
	}
	return users.outcome(answer, http.StatusNoContent, users.updateAction())
}

func (users Users) requirePasswordChange(userId string) error {
	user, err := users.readUser(userId)
	if err != nil {
		return err
	}
	err = user.requirePasswordChange()
	if err != nil {
		return err
	}
	return users.writeUser(userId, user)
}

func (users Users) logout(userId string) error {
	answer, err := users.admin.read(http.MethodPost, userPaths{}.Logout(userId))
	if err != nil {
		return err
	}
	return users.outcome(answer, http.StatusNoContent, users.logoutAction())
}

func (users Users) deleteCredentials(userId string, credentialIds []string) error {
	failures := []error{}
	for _, credentialId := range credentialIds {
		failures = append(failures, users.deleteCredential(userId, credentialId))
	}
	return errors.Join(failures...)
}

func (users Users) deleteCredential(userId string, credentialId string) error {
	answer, err := users.admin.read(http.MethodDelete, userPaths{}.Credential(userId, credentialId))
	if err != nil {
		return err
	}
	return users.outcome(answer, http.StatusNoContent, users.deleteCredentialAction())
}

func (Users) outcome(answer adminAnswer, success int, action string) error {
	switch answer.Status {
	case success:
		return nil
	case http.StatusNotFound:
		return UserMissing{}
	default:
		return AdminRefused{Status: answer.Status, Action: action}
	}
}

func (users Users) passwordOutcome(answer adminAnswer, success int, action string) error {
	switch answer.Status {
	case success:
		return nil
	case http.StatusBadRequest:
		return users.passwordRefused(answer.Content)
	case http.StatusNotFound:
		return UserMissing{}
	default:
		return AdminRefused{Status: answer.Status, Action: action}
	}
}

func (users Users) createOutcome(answer adminAnswer, username string) error {
	switch answer.Status {
	case http.StatusCreated:
		return nil
	case http.StatusConflict:
		return UserExists{Username: username}
	case http.StatusBadRequest:
		return users.passwordRefused(answer.Content)
	default:
		return AdminRefused{Status: answer.Status, Action: users.createAction()}
	}
}

func (users Users) deleteOutcome(answer adminAnswer) error {
	switch answer.Status {
	case http.StatusNoContent, http.StatusNotFound:
		return nil
	default:
		return AdminRefused{Status: answer.Status, Action: users.deleteAction()}
	}
}

func (users Users) passwordRefused(content []byte) PasswordRefused {
	var refusal refusalAnswer
	_ = users.admin.decode(content, &refusal)
	return PasswordRefused{Message: refusal.message()}
}

func (Users) passwordType() string {
	return "password"
}

func (Users) totpType() string {
	return "otp"
}

func (Users) operatorRole() string {
	return "mesail-operator"
}

func (Users) createAction() string {
	return "create the user"
}

func (Users) searchAction() string {
	return "search for the user"
}

func (Users) readAction() string {
	return "read the user"
}

func (Users) updateAction() string {
	return "update the user"
}

func (Users) resetAction() string {
	return "reset the password"
}

func (Users) logoutAction() string {
	return "sign the user out"
}

func (Users) credentialsAction() string {
	return "read the user credentials"
}

func (Users) deleteCredentialAction() string {
	return "remove the user credential"
}

func (Users) removePasskeysAction() string {
	return "remove the user passkeys"
}

func (Users) rolesAction() string {
	return "read the user roles"
}

func (Users) deleteAction() string {
	return "delete the user"
}
