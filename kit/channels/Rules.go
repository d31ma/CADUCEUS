package channels

import (
	"regexp"
	"slices"
	"strings"

	"mesail.dev/packages/oidc"
)

type Rules struct{}

func (rules Rules) CheckCreate(request CreateRequest) error {
	err := rules.checkName(request.Name)
	if err != nil {
		return err
	}
	err = rules.checkVisibility(request.Visibility)
	if err != nil {
		return err
	}
	return rules.checkTopic(&request.Topic)
}

func (rules Rules) CheckInvite(memberIds []string) error {
	err := rules.checkInviteSize(len(memberIds))
	if err != nil {
		return err
	}
	return rules.checkMemberIds(memberIds)
}

func (rules Rules) CheckUpdate(caller oidc.Caller, view ChannelView, request UpdateRequest) error {
	err := rules.checkAddressRight(rules.IsAdministrator(caller), request.EmailAddress)
	if err != nil {
		return err
	}
	err = rules.checkArchiveRight(rules.canArchive(caller, view), request.Archived)
	if err != nil {
		return err
	}
	err = rules.checkTopicRight(rules.canChangeTopic(caller, view), request.Topic)
	if err != nil {
		return err
	}
	err = rules.checkOpen(rules.changesArchivedTopic(view.Document, request))
	if err != nil {
		return err
	}
	return rules.checkTopic(request.Topic)
}

func (rules Rules) CheckWritable(document ChannelDocument) error {
	return rules.checkOpen(document.Archived)
}

func (rules Rules) AddressChange(domainId string, current string, requested *string) (AddressChange, error) {
	address := rules.normalized(current, requested)
	return AddressChange{Old: current, New: address}, rules.checkAddress(domainId, address)
}

func (rules Rules) IsAdministrator(caller oidc.Caller) bool {
	return slices.Contains(caller.Roles, rules.administrator())
}

func (rules Rules) checkName(name string) error {
	if rules.namePattern().MatchString(name) {
		return nil
	}
	return InvalidRequest{}.BadName()
}

func (rules Rules) checkVisibility(visibility string) error {
	if visibility == rules.public() || visibility == rules.private() {
		return nil
	}
	return InvalidRequest{}.BadVisibility()
}

func (rules Rules) checkTopic(topic *string) error {
	if topic == nil || len([]rune(*topic)) <= rules.maxTopicLength() {
		return nil
	}
	return InvalidRequest{}.TopicTooLong()
}

func (rules Rules) checkInviteSize(count int) error {
	if count >= 1 && count <= rules.maxInvites() {
		return nil
	}
	return InvalidRequest{}.BadInviteSize(rules.memberIdsField())
}

func (rules Rules) checkMemberIds(memberIds []string) error {
	if slices.Contains(memberIds, "") {
		return InvalidRequest{}.EmptyMemberId(rules.memberIdsField())
	}
	return nil
}

func (Rules) checkAddressRight(administrator bool, address *string) error {
	if address == nil || administrator {
		return nil
	}
	return Forbidden{}.AddressNeedsAdmin()
}

func (Rules) checkArchiveRight(allowed bool, archived *bool) error {
	if archived == nil || allowed {
		return nil
	}
	return Forbidden{}.ArchiveNeedsAdminOrCreator()
}

func (Rules) checkTopicRight(allowed bool, topic *string) error {
	if topic == nil || allowed {
		return nil
	}
	return Forbidden{}.TopicNeedsMember()
}

func (Rules) checkOpen(archived bool) error {
	if archived {
		return Conflict{}.Archived()
	}
	return nil
}

func (rules Rules) canArchive(caller oidc.Caller, view ChannelView) bool {
	return rules.IsAdministrator(caller) || view.Document.CreatedBy == caller.MemberId
}

func (rules Rules) canChangeTopic(caller oidc.Caller, view ChannelView) bool {
	return rules.IsAdministrator(caller) || view.Member
}

func (Rules) changesArchivedTopic(document ChannelDocument, request UpdateRequest) bool {
	switch {
	case request.Topic == nil:
		return false
	case request.Archived == nil:
		return document.Archived
	}
	return document.Archived && *request.Archived
}

func (Rules) normalized(current string, requested *string) string {
	if requested == nil {
		return current
	}
	return strings.ToLower(strings.TrimSpace(*requested))
}

func (rules Rules) checkAddress(domainId string, address string) error {
	if address == "" || rules.addressPattern(domainId).MatchString(address) {
		return nil
	}
	return InvalidRequest{}.BadAddress()
}

func (Rules) namePattern() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9_-]{1,80}$`)
}

func (Rules) addressPattern(domainId string) *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9._+-]{1,64}@` + regexp.QuoteMeta(domainId) + `$`)
}

func (Rules) public() string {
	return "public"
}

func (Rules) private() string {
	return "private"
}

func (Rules) administrator() string {
	return "DomainAdmin"
}

func (Rules) memberIdsField() string {
	return "memberIds"
}

func (Rules) maxTopicLength() int {
	return 250
}

func (Rules) maxInvites() int {
	return 100
}
