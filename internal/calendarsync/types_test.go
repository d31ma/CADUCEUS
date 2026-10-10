package calendarsync

import (
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestValidateSnapshotRequiresOwnedBoundedCalendarData(t *testing.T) {
	scope := accountsync.Scope{Owner: accounts.Owner{TenantID: "tenant", PrincipalID: "principal", Address: "alice@example.com"}, AccountID: "ext_AAAAAAAAAAAAAAAAAAAA"}
	now := time.Now().UTC()
	valid := Snapshot{PrincipalURL: "https://dav.example/principal", HomeURL: "https://dav.example/calendars", SyncedAt: now,
		Calendars: []Calendar{{StableID: "calendar_abcdefghijklmnopqrstuvwx", URL: "https://dav.example/calendars/work/", Name: "Work"}},
		Resources: []Resource{{ID: "caldav_abcdefghijklmnopqrstuvwx", AccountID: scope.AccountID, CalendarID: "calendar_abcdefghijklmnopqrstuvwx", Href: "https://dav.example/calendars/work/1.ics", ETag: `"1"`, RawICalendar: "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:1\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", RemoteUpdatedAt: now}}}
	if err := ValidateSnapshot(scope, valid); err != nil {
		t.Fatalf("valid snapshot: %v", err)
	}
	foreign := valid
	foreign.Resources = append([]Resource(nil), valid.Resources...)
	foreign.Resources[0].AccountID = "ext_BBBBBBBBBBBBBBBBBBBB"
	if err := ValidateSnapshot(scope, foreign); err != ErrInvalidSnapshot {
		t.Fatalf("foreign account error = %v", err)
	}
	malformed := valid
	malformed.Resources = append([]Resource(nil), valid.Resources...)
	malformed.Resources[0].RawICalendar = "not a calendar"
	if err := ValidateSnapshot(scope, malformed); err != ErrInvalidSnapshot {
		t.Fatalf("malformed calendar error = %v", err)
	}
}

func TestRemoteResourceIDIsAccountScopedAndStable(t *testing.T) {
	first := RemoteResourceID("ext_AAAAAAAAAAAAAAAAAAAA", "https://dav.example/1.ics")
	if first != RemoteResourceID("ext_AAAAAAAAAAAAAAAAAAAA", "https://dav.example/1.ics") || first == RemoteResourceID("ext_BBBBBBBBBBBBBBBBBBBB", "https://dav.example/1.ics") {
		t.Fatalf("resource identity is not stable and account scoped: %q", first)
	}
}
