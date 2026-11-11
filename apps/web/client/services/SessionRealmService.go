import (
	"regexp"
	"strings"
)

// @Service
type SessionRealmService struct{}

func (realm *SessionRealmService) DomainOf(email string) string {
	address := strings.ToLower(strings.TrimSpace(email))
	return realm.domainWhenValid(address, realm.emailPattern().MatchString(address))
}

func (realm *SessionRealmService) domainWhenValid(address string, valid bool) string {
	if !valid {
		return ""
	}
	return address[strings.LastIndex(address, "@")+1:]
}

func (realm *SessionRealmService) emailPattern() *regexp.Regexp {
	return regexp.MustCompile(`^[^@\s]+@([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)
}
