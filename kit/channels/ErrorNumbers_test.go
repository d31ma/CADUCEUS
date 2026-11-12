package channels

import (
	"errors"
	"testing"
)

type numbered interface {
	error
	Number() int
}

func TestEveryErrorCaseHasItsOwnNumber(t *testing.T) {
	cause := errors.New("cause")
	cases := []struct {
		err    numbered
		number int
		text   string
	}{
		{err: Unauthenticated{}.TokenRefused(), number: 2001, text: "the access token was refused"},
		{err: Forbidden{}.NotADomainMember(), number: 2002, text: "only members of a domain use its channels"},
		{err: InvalidRequest{}.BadName(), number: 2003, text: "a channel name has 1 to 80 lowercase letters, digits, - or _"},
		{err: InvalidRequest{}.BadVisibility(), number: 2004, text: "visibility is public or private"},
		{err: InvalidRequest{}.TopicTooLong(), number: 2005, text: "a topic has at most 250 characters"},
		{err: Conflict{}.NameUsed("releases"), number: 2006, text: "the channel name releases is already used in this domain"},
		{err: NotFound{}.BadChannelId(), number: 2007, text: "no such channel"},
		{err: NotFound{}.ChannelHidden(), number: 2008, text: "no such channel"},
		{err: InvalidRequest{}.BadInviteSize("memberIds"), number: 2009, text: "invite 1 to 100 members at a time"},
		{err: InvalidRequest{}.EmptyMemberId("memberIds"), number: 2010, text: "a member id is empty"},
		{err: NotFound{}.MemberOutsideDomain(), number: 2011, text: "no such member in this domain"},
		{err: Forbidden{}.AddressNeedsAdmin(), number: 2012, text: "only a domain admin can set a channel email address"},
		{err: Forbidden{}.ArchiveNeedsAdminOrCreator(), number: 2013, text: "only a domain admin or the channel creator can archive a channel"},
		{err: Forbidden{}.TopicNeedsMember(), number: 2014, text: "join the channel to change its topic"},
		{err: Conflict{}.Archived(), number: 2015, text: "the channel is archived and read-only"},
		{err: InvalidRequest{}.BadAddress(), number: 2016, text: "a channel email address is local-part@ the channel's own domain"},
		{err: InvalidRequest{}.AddressOutsideDomain("refused"), number: 2017, text: "refused"},
		{err: Duplicate{}.AddressTaken("taken"), number: 2018, text: "taken"},
		{err: Unavailable{}.DirectoryDown("down", cause), number: 2019, text: "down"},
		{err: Unavailable{}.StoreDown(cause), number: 2020, text: "the channel store is not available"},
		{err: Unavailable{}.StoreUnreadable(cause), number: 2021, text: "the channel store answered something unreadable"},
		{err: Unavailable{}.OutboxSettingsMissing(cause), number: 2022, text: "the outbox settings are missing"},
		{err: Unavailable{}.OutboxUnreadable(cause), number: 2023, text: "the outbox document cannot be read"},
		{err: NotFound{}.OutboxLetter("gone"), number: 2024, text: "gone"},
		{err: NotFound{}.OutboxDocument(), number: 2025, text: "no such outbox document"},
		{err: NotFound{}.OutboxCollection(), number: 2026, text: "no such outbox collection"},
	}
	for _, check := range cases {
		t.Run(check.text, func(t *testing.T) {
			wantEqual(t, check.err.Number(), check.number)
			wantEqual(t, check.err.Error(), check.text)
		})
	}
}

func TestInvalidRequestCasesNameTheirField(t *testing.T) {
	wantEqual(t, InvalidRequest{}.BadName().Field, "name")
	wantEqual(t, InvalidRequest{}.BadVisibility().Field, "visibility")
	wantEqual(t, InvalidRequest{}.TopicTooLong().Field, "topic")
	wantEqual(t, InvalidRequest{}.BadInviteSize("memberIds").Field, "memberIds")
	wantEqual(t, InvalidRequest{}.EmptyMemberId("memberIds").Field, "memberIds")
	wantEqual(t, InvalidRequest{}.BadAddress().Field, "emailAddress")
	wantEqual(t, InvalidRequest{}.AddressOutsideDomain("refused").Field, "emailAddress")
}

func TestAnUnavailableCaseKeepsItsCause(t *testing.T) {
	cause := errors.New("cause")
	wantTrue(t, errors.Is(Unavailable{}.StoreDown(cause), cause), "errors.Is does not find the cause")
}
