package attachments

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type AccessRules struct {
	Readers ReaderCheck
}

func (rules AccessRules) Check(caller oidc.Caller, record Record, question ReadQuestion) error {
	if rules.grantedHere(caller, record) || rules.Readers.MayRead(question.AttachmentId, question.Authorization) {
		return nil
	}
	return Missing{}
}

func (rules AccessRules) grantedHere(caller oidc.Caller, record Record) bool {
	return caller.MemberId == record.UploaderId || slices.Contains(caller.Roles, rules.serviceRole())
}

func (AccessRules) serviceRole() string {
	return "Service"
}
