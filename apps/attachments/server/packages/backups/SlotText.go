package backups

import "encoding/json"

type SlotText struct{}

func (text SlotText) Encode(slots []KeySlot) []string {
	stored := []string{}
	for _, slot := range slots {
		stored = append(stored, text.encoded(slot))
	}
	return stored
}

func (text SlotText) Decode(stored []string) []KeySlot {
	slots := []KeySlot{}
	for _, item := range stored {
		slots = append(slots, text.decoded(item)...)
	}
	return slots
}

func (SlotText) encoded(slot KeySlot) string {
	raw, _ := json.Marshal(slot)
	return string(raw)
}

func (SlotText) decoded(item string) []KeySlot {
	slot := KeySlot{}
	if json.Unmarshal([]byte(item), &slot) != nil {
		return []KeySlot{}
	}
	return []KeySlot{slot}
}
