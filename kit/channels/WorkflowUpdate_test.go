package channels

import (
	"errors"
	"strings"
	"testing"
)

func TestAMemberOfTheChannelChangesTheTopic(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.must(w.join("Bearer bo", general))
	updated, err := w.update("Bearer bo", general, UpdateRequest{Topic: text("release planning")})
	wantNoError(t, err)
	wantEqual(t, updated.Topic, "release planning")
	wantEqual(t, w.stored("example.com", general).Topic, "release planning")
}

func TestANonMemberCannotChangeTheTopic(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer bo", general, UpdateRequest{Topic: text("mine now")})
	wantError[*Forbidden](t, err)
}

func TestADomainAdminChangesTheTopicWithoutJoining(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer dee", general, UpdateRequest{Topic: text("set by admin")})
	wantNoError(t, err)
	wantEqual(t, w.stored("example.com", general).Topic, "set by admin")
}

func TestATopicThatIsTooLongIsRefusedOnUpdate(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer ana", general, UpdateRequest{Topic: text(strings.Repeat("t", 251))})
	wantField(t, err, "topic")
}

func TestTheCreatorArchivesTheChannelAndItIsPublishedOnce(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	first, firstErr := w.update("Bearer ana", general, UpdateRequest{Archived: flag(true)})
	_, secondErr := w.update("Bearer ana", general, UpdateRequest{Archived: flag(true)})
	wantNoError(t, firstErr)
	wantNoError(t, secondErr)
	wantEqual(t, first.Archived, true)
	wantEqual(t, w.stored("example.com", general).Archived, true)
	wantEqual(t, w.pushed("messaging", "channel.archived"), 1)
}

func TestAnotherMemberCannotArchive(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.must(w.join("Bearer bo", general))
	_, err := w.update("Bearer bo", general, UpdateRequest{Archived: flag(true)})
	wantError[*Forbidden](t, err)
}

func TestADomainAdminArchivesAnyVisibleChannel(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer dee", general, UpdateRequest{Archived: flag(true)})
	wantNoError(t, err)
	wantEqual(t, w.stored("example.com", general).Archived, true)
}

func TestAnArchivedChannelIsReadOnly(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer ana", general, UpdateRequest{Archived: flag(true)})
	w.must(err)
	_, err = w.update("Bearer ana", general, UpdateRequest{Topic: text("too late")})
	wantError[*Conflict](t, err)
}

func TestUnarchivingAndChangingTheTopicTogetherIsAllowed(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer ana", general, UpdateRequest{Archived: flag(true)})
	w.must(err)
	updated, err := w.update("Bearer ana", general, UpdateRequest{Archived: flag(false), Topic: text("back again")})
	wantNoError(t, err)
	wantEqual(t, updated.Archived, false)
	wantEqual(t, w.stored("example.com", general).Topic, "back again")
}

func TestArchivingAChannelWithAnUnreadableOutboxFails(t *testing.T) {
	w := newWorld(t)
	collection := Names{}.Channels("example.com")
	w.must(w.documents.Ensure(collection))
	id, err := w.documents.Insert(collection, ChannelDocument{DomainId: "example.com", Name: "broken", Visibility: "public", CreatedBy: "ana", PendingEvents: []string{"{"}})
	w.must(err)
	_, err = w.workflow.Update("Bearer ana", WireIds{}.ChannelId(id), UpdateRequest{Archived: flag(true)})
	wantTrue(t, err != nil, "an unreadable outbox was archived")
	wantEqual(t, w.stored("example.com", ChannelDocument{Id: id}).Archived, false)
}

func TestADomainAdminGivesTheChannelAnEmailAddress(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	updated, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text(" News@Example.com ")})
	wantNoError(t, err)
	wantEqual(t, updated.EmailAddress, "news@example.com")
	wantEqual(t, w.stored("example.com", general).EmailAddress, "news@example.com")
	wantEqual(t, w.directory.ClaimedBy("news@example.com"), "ch_"+general.Id)
}

func TestAMemberCannotSetTheEmailAddress(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer ana", general, UpdateRequest{EmailAddress: text("news@example.com")})
	wantError[*Forbidden](t, err)
}

func TestAnEmailAddressOfAnotherDomainOrWithoutALocalPartIsRefused(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, otherDomain := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.org")})
	_, noLocalPart := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("@example.com")})
	wantField(t, otherDomain, "emailAddress")
	wantField(t, noLocalPart, "emailAddress")
}

func TestAnAddressThatIsAlreadyUsedIsADuplicateAndNothingIsStored(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.directory.Take("bo@example.com")
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("bo@example.com")})
	wantError[*Duplicate](t, err)
	wantEqual(t, w.stored("example.com", general).EmailAddress, "")
}

func TestANewAddressReleasesTheOldOne(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("old@example.com")})
	w.must(err)
	_, err = w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("new@example.com")})
	wantNoError(t, err)
	wantEqual(t, w.directory.Released(), []string{"old@example.com"})
	wantEqual(t, w.directory.ClaimedBy("new@example.com"), "ch_"+general.Id)
}

func TestAnEmptyAddressClearsAndReleasesIt(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.com")})
	w.must(err)
	_, err = w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("")})
	wantNoError(t, err)
	wantEqual(t, w.directory.Released(), []string{"news@example.com"})
	wantEqual(t, w.stored("example.com", general).EmailAddress, "")
}

func TestTheSameAddressAgainChangesNothing(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.com")})
	w.must(err)
	w.directory.Take("news@example.com")
	_, err = w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("NEWS@example.com")})
	wantNoError(t, err)
	wantEqual(t, w.directory.Released(), []string{})
	wantEqual(t, w.stored("example.com", general).EmailAddress, "news@example.com")
}

func TestTheDirectoryFailingAClaimIsUnavailable(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	down := errors.New("directory is down")
	w.directory.Fail(down)
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.com")})
	wantError[*Unavailable](t, err)
	wantTrue(t, errors.Is(err, down), "the directory failure is not kept as the cause")
}

func TestTheDirectoryFailingAReleaseIsUnavailable(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.com")})
	w.must(err)
	w.directory.Fail(errors.New("directory is down"))
	_, err = w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("")})
	wantError[*Unavailable](t, err)
}

func TestAnAddressRefusedByTheDirectoryNamesTheAddressField(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.directory.Fail(AddressRefused{Address: "news@example.com"})
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.com")})
	wantField(t, err, "emailAddress")
}

func TestAChannelOfAnotherDomainIsNotFoundOnUpdate(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	_, err := w.update("Bearer eve", general, UpdateRequest{Topic: text("hello")})
	wantError[*NotFound](t, err)
}
