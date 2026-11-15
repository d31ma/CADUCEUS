package backups

import (
	"cmp"
	"regexp"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

type StartRules struct{}

func (rules StartRules) Check(request StartRequest, limit int64) error {
	return cmp.Or(
		rules.checkVersion(request), rules.checkSize(request), rules.checkSha256(request),
		SlotRules{}.Check(request.KeySlots), rules.checkDevice(request), rules.checkLimit(request, limit),
	)
}

func (StartRules) checkVersion(request StartRequest) error {
	if request.ExpectedVersion < 0 {
		return Invalid{}.ExpectedVersion()
	}
	return nil
}

func (StartRules) checkSize(request StartRequest) error {
	if request.Size < 1 {
		return Invalid{}.Size()
	}
	return nil
}

func (rules StartRules) checkSha256(request StartRequest) error {
	if !regexp.MustCompile(rules.sha256Pattern()).MatchString(request.Sha256) {
		return attachments.Invalid{}.Sha256()
	}
	return nil
}

func (rules StartRules) checkDevice(request StartRequest) error {
	if !regexp.MustCompile(rules.devicePattern()).MatchString(request.DeviceId) {
		return Invalid{}.DeviceId()
	}
	return nil
}

func (StartRules) checkLimit(request StartRequest, limit int64) error {
	if request.Size > limit {
		return blobs.TooLarge{Limit: limit}
	}
	return nil
}

func (StartRules) sha256Pattern() string {
	return `^[0-9a-f]{64}$`
}

func (StartRules) devicePattern() string {
	return `^[A-Za-z0-9_-]{1,128}$`
}
