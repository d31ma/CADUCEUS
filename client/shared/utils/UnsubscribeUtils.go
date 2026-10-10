import (
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// UnsubscribeUtils turns stored List-Unsubscribe metadata into one bounded
// reader action, as unsubscribe.js did. An empty Type means no action.
type UnsubscribeUtils struct{}

func (UnsubscribeUtils) BuildAction(email Email) UnsubscribeAction {
	oneClick := UnsubscribeUtils{}.oneClick(UnsubscribeUtils{}.secureWebUrl(email.UnsubscribeUrl), email.UnsubscribeOneClick)
	mailto := UnsubscribeUtils{}.mailtoAction(email.UnsubscribeMailto)
	opened := UnsubscribeUtils{}.opened(UnsubscribeUtils{}.secureWebUrl(email.UnsubscribeUrl))
	body := UnsubscribeUtils{}.opened(UnsubscribeUtils{}.secureWebUrl(email.UnsubscribeBodyUrl))
	return UnsubscribeUtils{}.first([]UnsubscribeAction{oneClick, mailto, opened, body})
}

func (UnsubscribeUtils) mailbox() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)
}

func (UnsubscribeUtils) maximumUrlLength() int { return 2048 }

func (UnsubscribeUtils) secureWebUrl(value string) string {
	address := UrlUtils{}.ParseWeb(value)
	tooLong := len(value) > UnsubscribeUtils{}.maximumUrlLength()
	if tooLong || !address.Valid || address.Scheme != "https" || address.HasCredentials {
		return ""
	}
	return address.Href
}

func (UnsubscribeUtils) oneClick(address string, oneClick bool) UnsubscribeAction {
	if address == "" || !oneClick {
		return UnsubscribeAction{}
	}
	return UnsubscribeAction{Type: "post", Url: address, To: []string{}}
}

func (UnsubscribeUtils) opened(address string) UnsubscribeAction {
	if address == "" {
		return UnsubscribeAction{}
	}
	return UnsubscribeAction{Type: "open", Url: address, To: []string{}}
}

func (UnsubscribeUtils) mailtoAction(value string) UnsubscribeAction {
	address := UrlUtils{}.ParseMailto(value)
	mailbox := strings.ToLower(strings.TrimSpace(address.Mailbox))
	tooLong := len(value) > UnsubscribeUtils{}.maximumUrlLength()
	valid := UnsubscribeUtils{}.mailbox().MatchString(mailbox)
	if tooLong || !address.Valid || address.HasAuthority || !valid {
		return UnsubscribeAction{}
	}
	return UnsubscribeAction{Type: "compose", To: []string{mailbox}, Subject: UnsubscribeUtils{}.boundedText(address.Subject), Body: UnsubscribeUtils{}.boundedText(address.Body)}
}

func (UnsubscribeUtils) maximumTextLength() int { return 2000 }

func (UnsubscribeUtils) boundedText(value string) string {
	tooLong := utf8.RuneCountInString(value) > UnsubscribeUtils{}.maximumTextLength()
	if tooLong || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func (UnsubscribeUtils) first(actions []UnsubscribeAction) UnsubscribeAction {
	found := slices.IndexFunc(actions, func(action UnsubscribeAction) bool { return action.Type != "" })
	return append(actions, UnsubscribeAction{To: []string{}})[UnsubscribeUtils{}.indexOr(found, len(actions))]
}

func (UnsubscribeUtils) indexOr(index int, fallback int) int {
	if index < 0 {
		return fallback
	}
	return index
}
