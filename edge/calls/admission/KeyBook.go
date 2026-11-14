package admission

import (
	"sync"
	"time"

	"mesail.dev/edge/calls/failures"
	"mesail.dev/packages/oidc"
)

type KeyBook struct {
	fetcher     oidc.KeyFetcher
	url         string
	clock       Clock
	lock        sync.Mutex
	keys        oidc.KeySet
	fetchedAt   time.Time
	attemptedAt time.Time
	missing     error
}

func NewKeyBook(fetcher oidc.KeyFetcher, url string, clock Clock) *KeyBook {
	return &KeyBook{fetcher: fetcher, url: url, clock: clock, missing: failures.Problems{}.InvalidToken()}
}

func (book *KeyBook) Key(keyId string) (oidc.JsonWebKey, error) {
	book.lock.Lock()
	defer book.lock.Unlock()
	book.refreshFor(keyId)
	return book.found(keyId)
}

func (book *KeyBook) refreshFor(keyId string) {
	if book.needsFetch(keyId) {
		book.fetch()
	}
}

func (book *KeyBook) needsFetch(keyId string) bool {
	return book.retryAllowed() && (book.stale() || !book.keys.Has(keyId))
}

func (book *KeyBook) retryAllowed() bool {
	return book.clock.Now().Sub(book.attemptedAt) >= book.retryInterval()
}

func (book *KeyBook) stale() bool {
	return book.clock.Now().Sub(book.fetchedAt) >= book.refreshInterval()
}

func (book *KeyBook) fetch() {
	book.attemptedAt = book.clock.Now()
	keys, err := book.fetcher.Fetch(book.url)
	book.remember(keys, err)
}

func (book *KeyBook) remember(keys oidc.KeySet, err error) {
	if err != nil {
		book.missing = failures.Problems{}.KeysUnavailable()
		return
	}
	book.keys = keys
	book.fetchedAt = book.attemptedAt
	book.missing = failures.Problems{}.InvalidToken()
}

func (book *KeyBook) found(keyId string) (oidc.JsonWebKey, error) {
	key, err := book.keys.Find(keyId)
	if err != nil {
		return oidc.JsonWebKey{}, book.missing
	}
	return key, nil
}

func (*KeyBook) retryInterval() time.Duration   { return 10 * time.Second }
func (*KeyBook) refreshInterval() time.Duration { return 10 * time.Minute }
