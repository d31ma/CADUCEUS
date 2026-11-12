package mailout

import "strings"

type Switch bool

func (value *Switch) UnmarshalJSON(raw []byte) error {
	text := strings.ToLower(strings.Trim(string(raw), `"`))
	on, known := value.meanings()[text]
	*value = Switch(on)
	return value.refused(known, string(raw))
}

func (value Switch) On() bool {
	return bool(value)
}

func (Switch) meanings() map[string]bool {
	return map[string]bool{"true": true, "false": false}
}

func (Switch) refused(known bool, raw string) error {
	if known {
		return nil
	}
	return InvalidSwitch{Value: raw}
}
