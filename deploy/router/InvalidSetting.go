package main

import "fmt"

type InvalidSetting struct {
	Name   string
	Reason string
}

func (invalid InvalidSetting) Error() string {
	return fmt.Sprintf("the setting %s is not valid: %s", invalid.Name, invalid.Reason)
}
