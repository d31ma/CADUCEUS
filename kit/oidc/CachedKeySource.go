package oidc

import "time"

type CachedKeySource struct {
	Fetcher KeyFetcher
	Cache   FileKeyCache
	Now     func() time.Time
}

func (source CachedKeySource) Key(jwksUrl string, keyId string) (JsonWebKey, error) {
	keys, err := source.current(jwksUrl, keyId)
	if err != nil {
		return JsonWebKey{}, err
	}
	return keys.Find(keyId)
}

func (source CachedKeySource) current(jwksUrl string, keyId string) (KeySet, error) {
	cached := source.Cache.Load(jwksUrl)
	if source.usable(cached, keyId) {
		return cached.Keys, nil
	}
	return source.refetch(jwksUrl)
}

func (source CachedKeySource) usable(cached CachedKeySet, keyId string) bool {
	age := source.Now().Unix() - cached.FetchedAt
	return age >= 0 && (age < source.minimumRefreshSeconds() || (age < source.maximumAgeSeconds() && cached.Keys.Has(keyId)))
}

func (source CachedKeySource) refetch(jwksUrl string) (KeySet, error) {
	keys, err := source.Fetcher.Fetch(jwksUrl)
	if err != nil {
		return KeySet{}, err
	}
	_ = source.Cache.Store(jwksUrl, CachedKeySet{FetchedAt: source.Now().Unix(), Keys: keys})
	return keys, nil
}

func (CachedKeySource) minimumRefreshSeconds() int64 {
	return 10
}

func (CachedKeySource) maximumAgeSeconds() int64 {
	return 300
}
