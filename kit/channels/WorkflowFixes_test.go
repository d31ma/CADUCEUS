package channels

import "testing"

func TestACreateWithoutOutboxSettingsStoresNothing(t *testing.T) {
	w := newWorld(t)
	w.configure(`{"yon":{}}`)
	_, err := w.workflow.Create("Bearer ana", CreateRequest{Name: "general", Visibility: "public"})
	wantError[*Unavailable](t, err)
	w.configure(settingsSnapshot())
	_, err = w.workflow.Create("Bearer ana", CreateRequest{Name: "general", Visibility: "public"})
	wantNoError(t, err)
}

func TestAFailedArchiveLeavesTheEmailAddressUnclaimed(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	w.configure(`{"yon":{}}`)
	_, err := w.update("Bearer dee", general, UpdateRequest{EmailAddress: text("news@example.com"), Archived: flag(true)})
	wantError[*Unavailable](t, err)
	wantEqual(t, w.directory.ClaimedBy("news@example.com"), "")
}

func TestArchivingAndChangingTheTopicTogetherIsAllowed(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	updated, err := w.update("Bearer ana", general, UpdateRequest{Topic: text("closed"), Archived: flag(true)})
	wantNoError(t, err)
	wantEqual(t, updated.Topic, "closed")
	wantEqual(t, updated.Archived, true)
}
