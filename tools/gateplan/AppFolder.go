package main

import "slices"

type AppFolder struct {
	Name        string
	KitPackages []string
}

func (folder AppFolder) Lists(kitPackage string) bool {
	return slices.Contains(folder.KitPackages, kitPackage)
}
