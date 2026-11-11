package reading

import (
	"cmp"
	"strings"
	"unicode/utf8"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type changeRules struct{}

func (rules changeRules) editable(caller oidc.Caller, message core.Message, body string) error {
	return cmp.Or(rules.author(caller, message), rules.present(message), rules.body(body))
}

func (changeRules) author(caller oidc.Caller, message core.Message) error {
	if caller.MemberId == "" || caller.MemberId != message.AuthorMemberId {
		return &core.Forbidden{Message: "only the author can change this message"}
	}
	return nil
}

func (changeRules) present(message core.Message) error {
	if message.DeletedAt != "" {
		return &core.Conflict{Message: "the message is deleted"}
	}
	return nil
}

func (rules changeRules) body(body string) error {
	if strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > rules.longestBody() {
		return &core.InvalidRequest{Message: "the body must have 1 to 40000 characters", Field: "body"}
	}
	return nil
}

func (changeRules) longestBody() int {
	return 40000
}
