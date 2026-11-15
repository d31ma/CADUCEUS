package backups

import (
	"errors"
	"strings"
	"testing"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

func expectInvalid(t *testing.T, err error, field string) {
	t.Helper()
	invalid := Invalid{}
	shared := attachments.Invalid{}
	if errors.As(err, &shared) && shared.Field == field {
		return
	}
	if !errors.As(err, &invalid) || invalid.Field != field {
		t.Fatalf("want invalid %s, got %v", field, err)
	}
}

func TestStartRulesAcceptAGoodRequest(t *testing.T) {
	if err := (StartRules{}).Check(startFor(0, "cipher", "phone-1_A"), 100); err != nil {
		t.Fatalf("good request refused: %v", err)
	}
}

func TestStartRulesRefuseEachBadField(t *testing.T) {
	cases := map[string]func(*StartRequest){
		"expectedVersion": func(request *StartRequest) { request.ExpectedVersion = -1 },
		"size":            func(request *StartRequest) { request.Size = 0 },
		"sha256":          func(request *StartRequest) { request.Sha256 = strings.ToUpper(request.Sha256) },
		"keySlots":        func(request *StartRequest) { request.KeySlots = nil },
		"deviceId":        func(request *StartRequest) { request.DeviceId = "a/b" },
	}
	for field, change := range cases {
		request := startFor(0, "cipher", "phone")
		change(&request)
		expectInvalid(t, StartRules{}.Check(request, 100), field)
	}
}

func TestStartRulesRefuseALargeBackup(t *testing.T) {
	err := StartRules{}.Check(startFor(0, "cipher", "phone"), 5)
	large := blobs.TooLarge{}
	if !errors.As(err, &large) || large.Limit != 5 {
		t.Fatalf("want too large, got %v", err)
	}
}

func TestSlotRulesAcceptRecoveryAndPasskeySlots(t *testing.T) {
	passkey := KeySlot{Kind: "passkey", Salt: encoded("salt"), CredentialId: encoded("credential"), WrappedKey: encoded("key") + "=="}
	if err := (SlotRules{}).Check([]KeySlot{recoverySlot(), passkey}); err != nil {
		t.Fatalf("good slots refused: %v", err)
	}
}

func TestSlotRulesRefuseBadSlots(t *testing.T) {
	cases := map[string]func(*KeySlot){
		"kind":          func(slot *KeySlot) { slot.Kind = "password" },
		"salt":          func(slot *KeySlot) { slot.Salt = "not base64!" },
		"empty key":     func(slot *KeySlot) { slot.WrappedKey = "" },
		"few rounds":    func(slot *KeySlot) { slot.Rounds = 1000 },
		"credential":    func(slot *KeySlot) { slot.CredentialId = encoded("credential") },
		"passkey no id": func(slot *KeySlot) { slot.Kind = "passkey" },
		"too big":       func(slot *KeySlot) { slot.WrappedKey = encoded(strings.Repeat("k", 800)) },
	}
	for name, change := range cases {
		slot := recoverySlot()
		change(&slot)
		if err := (SlotRules{}).Check([]KeySlot{slot}); err == nil {
			t.Fatalf("%s: bad slot accepted", name)
		}
	}
}

func TestSlotRulesRefuseMoreThanTenSlots(t *testing.T) {
	slots := []KeySlot{}
	for count := 0; count < 11; count++ {
		slots = append(slots, recoverySlot())
	}
	expectInvalid(t, SlotRules{}.Check(slots), "keySlots")
}

func TestErrorsNameTheirField(t *testing.T) {
	size := Invalid{}.Size().Error()
	replaced := Conflict{}.Replaced().Error()
	if size != "size: A backup must have at least one byte." {
		t.Fatal(size)
	}
	if !strings.HasPrefix(replaced, "expectedVersion: ") || (Missing{}).Error() == "" || (OtherDomain{}).Error() == "" {
		t.Fatal("error texts")
	}
}

func TestBlobNamesKeepTheDomainAndHideThePerson(t *testing.T) {
	key := BlobNames{}.New("example.com", "person-1", 3)
	if key.DomainId != "example.com" || strings.Contains(key.Name, "person-1") || !strings.Contains(key.Name, "-3-") {
		t.Fatalf("key %+v", key)
	}
	if err := (blobs.KeyRules{}).Check(key); err != nil {
		t.Fatalf("key breaks the blob key rules: %v", err)
	}
	if (BlobNames{}).Key(key.Path()) != key {
		t.Fatal("path does not round trip")
	}
}

func TestSlotTextRoundTripsAndSkipsBrokenText(t *testing.T) {
	stored := SlotText{}.Encode([]KeySlot{recoverySlot()})
	slots := SlotText{}.Decode(append(stored, "not json"))
	if len(stored) != 1 || len(slots) != 1 || slots[0] != recoverySlot() {
		t.Fatalf("stored %v slots %+v", stored, slots)
	}
}
