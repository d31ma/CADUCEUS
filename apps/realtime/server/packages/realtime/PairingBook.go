package realtime

import (
	"slices"
	"time"
)

type PairingBook struct {
	Storage Storage
	Now     func() time.Time
}

func (book PairingBook) Find(domainId string, pairingId string) (PairingRecord, bool, error) {
	record := PairingRecord{}
	found, err := book.shelf().Find(domainId, map[string]any{"pairingId": pairingId}, &record)
	return record, found, err
}

func (book PairingBook) Current(record PairingRecord) bool {
	return book.current(record)
}

func (book PairingBook) Open(domainId string, record PairingRecord) error {
	record.ExpiresAt = book.Now().UTC().Add(SignalLimits{}.PairingLifetime()).Format(time.RFC3339)
	return book.write(domainId, record)
}

func (book PairingBook) Join(domainId string, record PairingRecord, joinerDeviceId string) error {
	return book.shelf().Patch(domainId, record.ID, PairingJoin{JoinerDeviceId: joinerDeviceId})
}

func (book PairingBook) Peers(domainId string, deviceId string) ([]string, error) {
	opened, err := book.matching(domainId, "openerDeviceId", deviceId)
	if err != nil {
		return nil, err
	}
	joined, err := book.matching(domainId, "joinerDeviceId", deviceId)
	if err != nil {
		return nil, err
	}
	return book.peersOf(append(opened, joined...), deviceId), nil
}

func (book PairingBook) write(domainId string, record PairingRecord) error {
	if record.ID == "" {
		return book.shelf().Insert(domainId, record)
	}
	id := record.ID
	record.ID = ""
	return book.shelf().Patch(domainId, id, record)
}

func (book PairingBook) matching(domainId string, field string, deviceId string) ([]PairingRecord, error) {
	found := []PairingRecord{}
	err := book.shelf().All(domainId, map[string]any{field: deviceId}, &found)
	return slices.DeleteFunc(found, book.unusable), err
}

func (book PairingBook) unusable(record PairingRecord) bool {
	return record.JoinerDeviceId == "" || !book.current(record)
}

func (PairingBook) peersOf(records []PairingRecord, deviceId string) []string {
	peers := []string{}
	for _, record := range records {
		peers = append(peers, record.OpenerDeviceId, record.JoinerDeviceId)
	}
	return MemberSet{}.Without(peers, deviceId)
}

func (book PairingBook) current(record PairingRecord) bool {
	expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt)
	return err == nil && book.Now().Before(expiresAt)
}

func (book PairingBook) shelf() DomainShelf {
	return DomainShelf{Storage: book.Storage, Base: "pairings"}
}
