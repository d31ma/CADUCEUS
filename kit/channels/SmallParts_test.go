package channels

import (
	"errors"
	"testing"

	"mesail.dev/packages/outbox"
)

func TestCollectionsAreNamedWithTheDomainKey(t *testing.T) {
	wantEqual(t, Names{}.Channels("example.com"), "channels-a379a6f6eeafb9a5")
	wantEqual(t, Names{}.Memberships("example.com"), "channel-members-a379a6f6eeafb9a5")
	wantEqual(t, Names{}.Registry(), "outbox-collections")
}

func TestWireIdsHaveTheirPrefixes(t *testing.T) {
	stored, err := WireIds{}.StoredId("ch_00000000000001")
	wantNoError(t, err)
	wantEqual(t, stored, "00000000000001")
	wantEqual(t, WireIds{}.ChannelId("7"), "ch_7")
	wantEqual(t, WireIds{}.ConversationId("7"), "cnv_7")
}

func TestAnIdWithoutTheChannelPrefixOrWithoutABodyIsNotFound(t *testing.T) {
	_, otherKind := WireIds{}.StoredId("cnv_7")
	_, empty := WireIds{}.StoredId("ch_")
	wantError[*NotFound](t, otherKind)
	wantError[*NotFound](t, empty)
}

func TestSettingsComeFromTheYonApplicationSnapshot(t *testing.T) {
	settings := SettingsSource{}.FromSnapshot(settingsSnapshot())
	wantEqual(t, settings.DirectoryUrl, "http://127.0.0.1:1")
	wantEqual(t, settings.RelayUrl, "http://127.0.0.1:1/relay")
	wantEqual(t, settings.Outbox.Producer, "channels")
	wantEqual(t, len(settings.Outbox.Subscribers), 3)
	wantEqual(t, settings.OutboxError, nil)
}

func TestSettingsAreReadFromTheEnvironment(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", settingsSnapshot())
	wantEqual(t, SettingsSource{}.FromEnvironment().DirectoryUrl, "http://127.0.0.1:1")
}

func TestAMissingSnapshotGivesEmptySettingsWithAnOutboxError(t *testing.T) {
	settings := SettingsSource{}.FromSnapshot("")
	wantEqual(t, settings.DirectoryUrl, "")
	wantTrue(t, settings.OutboxError != nil, "missing outbox settings were accepted")
}

func TestEveryErrorSaysWhatWentWrong(t *testing.T) {
	cause := errors.New("cause")
	cases := []struct {
		err  error
		want string
	}{
		{err: &Conflict{Message: "conflict"}, want: "conflict"},
		{err: &Duplicate{Message: "duplicate"}, want: "duplicate"},
		{err: &Forbidden{Message: "forbidden"}, want: "forbidden"},
		{err: &InvalidRequest{Message: "invalid", Field: "name"}, want: "invalid"},
		{err: &NotFound{Message: "not found"}, want: "not found"},
		{err: &Unauthenticated{Message: "unauthenticated"}, want: "unauthenticated"},
		{err: &Unavailable{Message: "unavailable", Cause: cause}, want: "unavailable"},
		{err: AddressRefused{Address: "x@example.org"}, want: "the address x@example.org is not in the channel's domain"},
		{err: AddressTaken{Address: "bo@example.com"}, want: "the address bo@example.com already belongs to a member, an alias or another channel"},
		{err: DirectoryFailed{Action: "the member lookup", Status: 500}, want: "the directory refused the member lookup with status 500"},
		{err: SweepRefused{Status: 502}, want: "the outbox relay route answered 502"},
	}
	for _, check := range cases {
		t.Run(check.want, func(t *testing.T) {
			wantEqual(t, check.err.Error(), check.want)
		})
	}
}

func TestUnavailableKeepsItsCause(t *testing.T) {
	cause := outbox.MissingSettings{}
	failure := &Unavailable{Message: "unavailable", Cause: cause}
	wantEqual(t, failure.Unwrap(), error(cause))
	wantTrue(t, errors.Is(failure, cause), "errors.Is does not find the cause")
}
