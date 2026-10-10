package contactsync

import (
	"strings"
	"testing"
	"time"

	"github.com/d31ma/caduceus/internal/accounts"
	"github.com/d31ma/caduceus/internal/accountsync"
)

func TestValidateSnapshotRequiresExactAccountAndBookReferences(t *testing.T) {
	scope := accountsync.Scope{Owner: accounts.Owner{TenantID: "tenant-a", PrincipalID: "principal-a", Address: "alice@example.com"}, AccountID: "ext_AAAAAAAAAAAAAAAAAAAA"}
	now := time.Now().UTC()
	valid := Snapshot{PrincipalURL: "https://dav.example/principals/alice/", HomeURL: "https://dav.example/books/alice/", SyncedAt: now,
		AddressBooks: []AddressBook{{StableID: "book_a", URL: "https://dav.example/books/alice/default/", Name: "Contacts"}},
		Contacts:     []Contact{{ID: "carddav_a", AccountID: scope.AccountID, AddressBookID: "book_a", Href: "/books/alice/default/a.vcf", ETag: `"7"`, Name: "Alice", RawVCard: "BEGIN:VCARD\r\nEND:VCARD\r\n", RemoteUpdatedAt: now}},
	}
	if err := ValidateSnapshot(scope, valid); err != nil {
		t.Fatalf("valid snapshot: %v", err)
	}
	foreign := valid
	foreign.Contacts = append([]Contact(nil), valid.Contacts...)
	foreign.Contacts[0].AccountID = "ext_BBBBBBBBBBBBBBBBBBBB"
	if err := ValidateSnapshot(scope, foreign); err != ErrInvalidSnapshot {
		t.Fatalf("foreign account error=%v", err)
	}
	missingBook := valid
	missingBook.Contacts = append([]Contact(nil), valid.Contacts...)
	missingBook.Contacts[0].AddressBookID = "book_missing"
	if err := ValidateSnapshot(scope, missingBook); err != ErrInvalidSnapshot {
		t.Fatalf("missing book error=%v", err)
	}
}

func TestValidateMutationAcceptsBoundedCreateAndRejectsIncompleteIdentity(t *testing.T) {
	request := MutationRequest{
		ID:             "cmut_abcdefghijklmnopqrstuvwx",
		IdempotencyKey: "create-1",
		Kind:           MutationCreate,
		Desired: Contact{
			AccountID:     "ext_AAAAAAAAAAAAAAAAAAAA",
			AddressBookID: "book_abcdefghijklmnopqrstuvwx",
			Href:          "https://dav.example/books/alice/default/caduceus-1.vcf",
			UID:           "7f6f4f36-8ab5-4f4d-a644-9ef737806e6e",
			Name:          "Alice Example",
		},
	}
	request.ContactID = RemoteContactID(request.Desired.AccountID, request.Desired.Href)
	request.Desired.ID = request.ContactID
	if err := ValidateMutation(request); err != nil {
		t.Fatalf("valid create: %v", err)
	}
	if got := RemoteContactID(request.Desired.AccountID, request.Desired.Href); got != request.ContactID || !strings.HasPrefix(got, "carddav_") {
		t.Fatalf("RemoteContactID() = %q, want %q", got, request.ContactID)
	}
	missingBook := request
	missingBook.Desired.AddressBookID = ""
	if err := ValidateMutation(missingBook); err != ErrInvalidMutation {
		t.Fatalf("missing address book error = %v", err)
	}
	withETag := request
	withETag.ExpectedETag = `"7"`
	if err := ValidateMutation(withETag); err != ErrInvalidMutation {
		t.Fatalf("create ETag error = %v", err)
	}
	oversizedValue := request
	oversizedValue.Desired.Emails = []Value{{Value: strings.Repeat("a", 2049)}}
	if err := ValidateMutation(oversizedValue); err != ErrInvalidMutation {
		t.Fatalf("oversized contact value error = %v", err)
	}
}
