package oidc

type RealmRoles struct{}

func (RealmRoles) Member() string   { return "mesail-member" }
func (RealmRoles) Admin() string    { return "mesail-admin" }
func (RealmRoles) Operator() string { return "mesail-operator" }
func (RealmRoles) Service() string  { return "mesail-service" }
