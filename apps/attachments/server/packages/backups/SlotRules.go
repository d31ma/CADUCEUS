package backups

import (
	"cmp"
	"encoding/base64"
	"encoding/json"
	"slices"
	"strings"
)

type SlotRules struct{}

func (rules SlotRules) Check(slots []KeySlot) error {
	return cmp.Or(rules.checkCount(slots), rules.checkEach(slots))
}

func (SlotRules) checkCount(slots []KeySlot) error {
	if len(slots) < 1 || len(slots) > (Limits{}).MostKeySlots() {
		return Invalid{}.KeySlots()
	}
	return nil
}

func (rules SlotRules) checkEach(slots []KeySlot) error {
	for _, slot := range slots {
		err := rules.checkSlot(slot)
		if err != nil {
			return err
		}
	}
	return nil
}

func (rules SlotRules) checkSlot(slot KeySlot) error {
	if !rules.valid(slot) {
		return Invalid{}.KeySlots()
	}
	return nil
}

func (rules SlotRules) valid(slot KeySlot) bool {
	checks := []bool{
		rules.knownKind(slot), rules.encoded(slot.Salt), rules.encoded(slot.WrappedKey),
		rules.enoughRounds(slot), rules.credentialFits(slot), rules.small(slot),
	}
	return !slices.Contains(checks, false)
}

func (SlotRules) knownKind(slot KeySlot) bool {
	limits := Limits{}
	return slices.Contains([]string{limits.RecoveryKind(), limits.PasskeyKind()}, slot.Kind)
}

func (SlotRules) encoded(text string) bool {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(text, "="))
	return err == nil && len(decoded) > 0
}

func (rules SlotRules) enoughRounds(slot KeySlot) bool {
	return slot.Rounds >= rules.leastRounds(slot.Kind)
}

func (SlotRules) leastRounds(kind string) int64 {
	if kind == (Limits{}).RecoveryKind() {
		return Limits{}.RecoveryRounds()
	}
	return 0
}

func (rules SlotRules) credentialFits(slot KeySlot) bool {
	return rules.encoded(slot.CredentialId) == (slot.Kind == Limits{}.PasskeyKind())
}

func (SlotRules) small(slot KeySlot) bool {
	text, _ := json.Marshal(slot)
	return len(text) <= Limits{}.LargestKeySlot()
}
