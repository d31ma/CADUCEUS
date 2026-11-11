package main

import "fmt"

type MissingSetting struct {
	Name string
}

func (missing MissingSetting) Error() string {
	return fmt.Sprintf("the setting %s is required", missing.Name)
}
