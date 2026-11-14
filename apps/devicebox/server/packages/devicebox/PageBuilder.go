package devicebox

import (
	"slices"
	"strconv"
)

type PageBuilder struct {
	store Store
}

func NewPageBuilder(store Store) PageBuilder {
	return PageBuilder{store: store}
}

func (builder PageBuilder) Page(domainId string, rows []WaitingRow, size int) (EnvelopePage, error) {
	sorted := builder.sorted(rows)
	page := EnvelopePage{Envelopes: []WaitingEnvelope{}}
	ciphertextBytes := 0
	taken := 0
	for ; taken < len(sorted) && builder.hasRoom(ciphertextBytes, taken); taken++ {
		envelope, found, err := builder.envelope(domainId, sorted[taken])
		if err != nil {
			return EnvelopePage{}, err
		}
		page.Envelopes = append(page.Envelopes, builder.ifFound(envelope, found)...)
		ciphertextBytes += len(envelope.Ciphertext)
	}
	page.Next = builder.next(sorted, taken, size)
	return page, nil
}

func (PageBuilder) sorted(rows []WaitingRow) []WaitingRow {
	sorted := slices.Clone(rows)
	slices.SortStableFunc(sorted, func(left WaitingRow, right WaitingRow) int {
		return int(left.StoredUs - right.StoredUs)
	})
	return sorted
}

func (PageBuilder) hasRoom(ciphertextBytes int, taken int) bool {
	return taken == 0 || ciphertextBytes < Limits{}.PageCiphertextBytes()
}

func (builder PageBuilder) envelope(domainId string, row WaitingRow) (WaitingEnvelope, bool, error) {
	body, found, err := builder.store.Body(domainId, row.BodyId)
	envelope := WaitingEnvelope{
		EnvelopeId:     row.EnvelopeId,
		SenderDeviceId: row.SenderDeviceId,
		SenderPersonId: row.SenderPersonId,
		Kind:           row.Kind,
		Ciphertext:     body.Ciphertext,
		StoredAt:       row.StoredAt,
	}
	return envelope, found, err
}

func (PageBuilder) ifFound(envelope WaitingEnvelope, found bool) []WaitingEnvelope {
	if !found {
		return nil
	}
	return []WaitingEnvelope{envelope}
}

func (PageBuilder) next(sorted []WaitingRow, taken int, size int) string {
	if taken == 0 || (taken == len(sorted) && len(sorted) < size) {
		return ""
	}
	return strconv.FormatInt(sorted[taken-1].StoredUs, 10)
}
