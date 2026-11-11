package oidc

type AuthorizedParties struct{}

func (AuthorizedParties) Web() string      { return "mesail-web" }
func (AuthorizedParties) Service() string  { return "mesail-service" }
func (AuthorizedParties) Identity() string { return "mesail-identity" }
