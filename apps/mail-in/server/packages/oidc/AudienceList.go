package oidc

import "encoding/json"

type AudienceList []string

func (audience *AudienceList) UnmarshalJSON(raw []byte) error {
	values, err := audience.decode(raw)
	*audience = values
	return err
}

func (*AudienceList) decode(raw []byte) ([]string, error) {
	many := []string{}
	if json.Unmarshal(raw, &many) == nil {
		return many, nil
	}
	one := ""
	err := json.Unmarshal(raw, &one)
	return []string{one}, err
}
