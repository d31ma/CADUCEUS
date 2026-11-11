package search

import "encoding/json"

type Json struct{}

func (Json) Copy(source any, target any) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return InvalidRequest{Message: "the value cannot be written as JSON", Field: "body"}
	}
	return Json{}.decoded(json.Unmarshal(encoded, target))
}

func (Json) decoded(failure error) error {
	if failure != nil {
		return InvalidRequest{Message: "the value does not have the expected fields", Field: "body"}
	}
	return nil
}
