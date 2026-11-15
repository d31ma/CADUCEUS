package main

import "fmt"

type ChangeList struct {
	Paths   []string
	Failure error
	Base    string
}

func (changes ChangeList) failureReason() string {
	if changes.Failure == nil {
		return ""
	}
	return fmt.Sprintf("the changed files against %s could not be listed: %v", changes.Base, changes.Failure)
}

func (changes ChangeList) emptyReason() string {
	if len(changes.Paths) > 0 {
		return ""
	}
	return "no file differs from " + changes.Base
}
