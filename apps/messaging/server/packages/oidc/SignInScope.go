package oidc

type SignInScope struct{}

func (scope SignInScope) For(kept bool) string {
	if kept {
		return scope.plain() + " " + scope.offline()
	}
	return scope.plain()
}

func (SignInScope) plain() string {
	return "openid"
}

func (SignInScope) offline() string {
	return "offline_access"
}
