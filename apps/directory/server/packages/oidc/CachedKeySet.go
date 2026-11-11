package oidc

type CachedKeySet struct {
	FetchedAt int64  `json:"fetchedAt"`
	Keys      KeySet `json:"keys"`
}
