package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type Owner struct {
	Name   string
	First  int
	Last   int
	Folder string
}

func (Owner) FromBlock(first int, last int, owner string) Owner {
	return Owner{Name: path.Base(owner), First: first, Last: last, Folder: Owner{}.folder(owner)}
}

func (owner Owner) Holds(number int) bool {
	return owner.First <= number && number <= owner.Last
}

func (owner Owner) IsKit() bool {
	return owner.First == 0
}

func (owner Owner) Block() string {
	return fmt.Sprintf("%s (MS%04d-MS%04d)", owner.Name, owner.First, owner.Last)
}

func (Owner) folder(owner string) string {
	switch {
	case owner == "kit":
		return owner
	case strings.Contains(owner, "/"):
		return filepath.FromSlash(owner)
	}
	return filepath.Join("apps", owner, "server")
}
