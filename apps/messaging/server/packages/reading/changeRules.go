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
		return core.Forbidden{}.NotTheAuthor()
	}
	return nil
}

func (changeRules) present(message core.Message) error {
	if message.DeletedAt != "" {
		return core.Conflict{}.MessageDeleted()
	}
	return nil
}

func (changeRules) sameVersion(message core.Message, ifVersion int) bool {
	if ifVersion == 0 {
		return true
	}
	return ifVersion == core.Versions{}.Current(message)
}

func (rules changeRules) body(body string) error {
	if strings.TrimSpace(body) == "" || utf8.RuneCountInString(body) > rules.longestBody() {
		return core.InvalidRequest{}.EditedBodySize()
	}
	return nil
}

func (changeRules) longestBody() int {
	return 40000
}
