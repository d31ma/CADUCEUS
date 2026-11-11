import "strings"

// @Service
type AdminLoginService struct{}

func (login *AdminLoginService) Cleaned(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func (login *AdminLoginService) Missing(text string) bool {
	return login.emptyOrOpen(login.Cleaned(text))
}

func (login *AdminLoginService) UsernameOf(text string) string {
	username, _ := login.parts(text)
	return username
}

func (login *AdminLoginService) DomainOf(text string) string {
	_, domainId := login.parts(text)
	return domainId
}

func (login *AdminLoginService) Hint(text string) string {
	return login.hintFor(login.DomainOf(text), login.Cleaned(text))
}

func (login *AdminLoginService) Forbidden(text string) string {
	return login.forbiddenFor(login.DomainOf(text))
}

func (login *AdminLoginService) emptyOrOpen(cleaned string) bool {
	return cleaned == "" || strings.HasSuffix(cleaned, login.at())
}

func (login *AdminLoginService) parts(text string) (string, string) {
	cleaned := login.Cleaned(text)
	at := strings.LastIndex(cleaned, login.at())
	return login.split(cleaned, at)
}

func (login *AdminLoginService) split(cleaned string, at int) (string, string) {
	if at < 0 {
		return cleaned, ""
	}
	return cleaned[:at], cleaned[at+1:]
}

func (login *AdminLoginService) hintFor(domainId string, cleaned string) string {
	if cleaned == "" {
		return "Your username, like ana, or your full address."
	}
	return login.domainHint(domainId)
}

func (login *AdminLoginService) domainHint(domainId string) string {
	return map[bool]string{true: "Opens your default domain, or the whole install for an Operator.", false: "Opens " + domainId + "."}[domainId == ""]
}

func (login *AdminLoginService) forbiddenFor(domainId string) string {
	if domainId == "" {
		return "noDomain"
	}
	return "notMember"
}

func (login *AdminLoginService) at() string {
	return "@"
}
