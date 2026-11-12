package search

import "encoding/json"

type Json struct{}

func (Json) Copy(source any, target any) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return InvalidRequest{}.NotWritable()
	}
	return Json{}.decoded(json.Unmarshal(encoded, target))
}

func (Json) decoded(failure error) error {
	if failure != nil {
		return InvalidRequest{}.MissingFields()
	}
	return nil
}
