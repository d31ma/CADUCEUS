package channels

import (
	"errors"
	"testing"
	"time"
)

func TestEveryMemberOfTheDomainSeesAPublicChannel(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	wantEqual(t, w.listed("Bearer bo"), []string{"general"})
	wantEqual(t, w.listed("Bearer dee"), []string{"general"})
}

func TestAPrivateChannelIsListedOnlyForItsMembers(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	w.create("Bearer ana", "secret", "private")
	wantEqual(t, w.listed("Bearer ana"), []string{"general", "secret"})
	wantEqual(t, w.listed("Bearer bo"), []string{"general"})
}

func TestAChannelIsNeverListedInAnotherDomain(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	wantEqual(t, w.listed("Bearer eve"), []string{})
}

func TestAPrivateChannelIsNotListedAfterLeaving(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	w.must(w.invite("Bearer ana", secret, "bo"))
	wantEqual(t, w.listed("Bearer bo"), []string{"secret"})
	w.must(w.leave("Bearer bo", secret))
	wantEqual(t, w.listed("Bearer bo"), []string{})
}

func TestAMemberJoinsAPublicChannel(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.must(w.join("Bearer bo", general))
	wantEqual(t, w.membership("example.com", general, "bo").MemberId, "bo")
	wantEqual(t, w.pushed("messaging", "channel.member.added"), 2)
}

func TestJoiningTwiceAddsOneEvent(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.must(w.join("Bearer bo", general))
	w.must(w.join("Bearer bo", general))
	wantEqual(t, w.pushed("realtime", "channel.member.added"), 2)
}

func TestJoiningAPrivateChannelIsNotFound(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	wantError[*NotFound](t, w.join("Bearer bo", secret))
}

func TestJoiningAChannelOfAnotherDomainIsNotFound(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	wantError[*NotFound](t, w.join("Bearer eve", general))
}

func TestJoiningAnUnknownChannelOrAnIdOfAnotherKindIsNotFound(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	wantError[*NotFound](t, w.workflow.Join("Bearer bo", "ch_99999999999999"))
	wantError[*NotFound](t, w.workflow.Join("Bearer bo", "cnv_"+general.Id))
	wantError[*NotFound](t, w.workflow.Join("Bearer bo", "ch_"))
}

func TestJoiningAnArchivedChannelIsAConflict(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer ana", general, UpdateRequest{Archived: flag(true)})
	w.must(err)
	wantError[*Conflict](t, w.join("Bearer bo", general))
}

func TestLeavingRemovesTheMemberAndPublishes(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.must(w.join("Bearer bo", general))
	w.must(w.leave("Bearer bo", general))
	wantEqual(t, w.membership("example.com", general, "bo").Left, true)
	wantEqual(t, w.pushed("search", "channel.member.removed"), 1)
}

func TestLeavingWithoutBeingAMemberChangesNothing(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.must(w.leave("Bearer bo", general))
	w.must(w.join("Bearer bo", general))
	w.must(w.leave("Bearer bo", general))
	w.must(w.leave("Bearer bo", general))
	wantEqual(t, w.pushed("messaging", "channel.member.removed"), 1)
}

func TestLeavingAPrivateChannelOfOthersIsNotFound(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	wantError[*NotFound](t, w.leave("Bearer bo", secret))
}

func TestRejoiningPublishesTheMemberAddedAgain(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.clock = w.clock.Add(time.Minute)
	w.must(w.join("Bearer bo", general))
	w.must(w.leave("Bearer bo", general))
	w.clock = w.clock.Add(time.Minute)
	w.must(w.join("Bearer bo", general))
	membership := w.membership("example.com", general, "bo")
	wantEqual(t, membership.Left, false)
	wantEqual(t, membership.JoinedAt, "2026-11-10T12:02:00Z")
	wantEqual(t, w.pushed("messaging", "channel.member.added"), 3)
}

func TestAMemberOfTheChannelInvitesMembersOfTheSameDomain(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	w.must(w.invite("Bearer ana", secret, "bo", "cy"))
	wantEqual(t, w.listed("Bearer bo"), []string{"secret"})
	wantEqual(t, w.listed("Bearer cy"), []string{"secret"})
}

func TestInvitingAMemberOfAnotherDomainIsRefusedAndAddsNobody(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	wantError[*NotFound](t, w.invite("Bearer ana", secret, "bo", "eve"))
	wantEqual(t, w.listed("Bearer bo"), []string{})
}

func TestANonMemberCannotInviteToAPrivateChannel(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	wantError[*NotFound](t, w.invite("Bearer bo", secret, "cy"))
}

func TestAnInviteNeedsOneToOneHundredMemberIdsThatAreNotEmpty(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	tooMany := make([]string, 101)
	wantField(t, w.invite("Bearer ana", secret), "memberIds")
	wantField(t, w.workflow.Invite("Bearer ana", WireIds{}.ChannelId(secret.Id), tooMany), "memberIds")
	wantField(t, w.invite("Bearer ana", secret, "bo", ""), "memberIds")
}

func TestAnInviteToAnArchivedChannelIsAConflict(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	_, err := w.update("Bearer ana", secret, UpdateRequest{Archived: flag(true)})
	w.must(err)
	wantError[*Conflict](t, w.invite("Bearer ana", secret, "bo"))
}

func TestAnInviteFailsWhenTheDirectoryIsDown(t *testing.T) {
	w := newWorld(t)
	secret := w.create("Bearer ana", "secret", "private")
	down := errors.New("directory is down")
	w.directory.Fail(down)
	err := w.invite("Bearer ana", secret, "bo")
	wantError[*Unavailable](t, err)
	wantTrue(t, errors.Is(err, down), "the directory failure is not kept as the cause")
}
