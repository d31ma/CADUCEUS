package oidc

type ServiceTokenSource interface {
	CacheKey() string
	Fetch() (TokenGrant, error)
}
