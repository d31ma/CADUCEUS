package keycloak

import "encoding/json"

type mustChangePassword struct{}

func (mustChangePassword) Key() string {
	return "mesail.mustChangePassword"
}

func (mustChangePassword) Values() []string {
	return []string{"true"}
}

func (mustChangePassword) Raw() json.RawMessage {
	return json.RawMessage(`["true"]`)
}
